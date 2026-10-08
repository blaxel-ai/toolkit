package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/monitor"
)

const (
	deployEvidenceLines = 20
	deployEvidenceBytes = 8 * 1024
	noSpecificCause     = "No more specific cause was returned."
)

// PrivateFailureDetail matches infrastructure details. Older event records can
// contain raw infrastructure errors. Never promote those details into a
// terminal error when reading an existing build history.
var PrivateFailureDetail = regexp.MustCompile(`(?i)\b(aws|amazon|dynamodb|eventbridge|lambda|stripe|cloudflare|workos|authkit|depot|smithy|grpc|requestid|request-id|stacktrace|secretsmanager|cloudfront)\b|(?:^|[\s:])(s3|sqs|kms|ses|sfn)(?:[\s:]|$)|step[ -]?functions|arn:|https?://|\.internal\b|\.(?:amazonaws\.com|bl\.run)\b|operation error|api error|\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// Event is one entry of a resource's event history.
type Event struct {
	Type     string `json:"type"`
	Status   string `json:"status"`
	Time     string `json:"time"`
	Message  string `json:"message"`
	Revision string `json:"revision,omitempty"`
}

// LatestEvent selects the newest event (including a newer non-failure),
// never an older convenient failure. Malformed history cannot establish order.
func LatestEvent(raw json.RawMessage, typePrefix string) (Event, bool) {
	var events []Event
	if json.Unmarshal(raw, &events) != nil {
		return Event{}, false
	}
	latest := -1
	var latestTime time.Time
	for i, event := range events {
		if typePrefix != "" && !strings.HasPrefix(event.Type, typePrefix) {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, event.Time)
		if err != nil {
			return Event{}, false
		}
		if latest == -1 || !timestamp.Before(latestTime) {
			latest, latestTime = i, timestamp
		}
	}
	if latest == -1 {
		return Event{}, false
	}
	return events[latest], true
}

// Diagnostics explains a deploy that bl deploy --wait saw fail or could not
// confirm.
type Diagnostics struct {
	Code    string   `json:"code" yaml:"code"`
	Phase   string   `json:"phase" yaml:"phase"`
	Cause   string   `json:"cause" yaml:"cause"`
	Step    string   `json:"step,omitempty" yaml:"step,omitempty"`
	Source  string   `json:"source,omitempty" yaml:"source,omitempty"`
	LogTail []string `json:"logTail,omitempty" yaml:"logTail,omitempty"`
	Next    Next     `json:"next" yaml:"next"`
}

type Next struct {
	Message string   `json:"message" yaml:"message"`
	Command []string `json:"command" yaml:"command"`
}

var (
	deployTerminalEscape = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)")
	deployDockerfileLine = regexp.MustCompile(`(?m)^Dockerfile:\d+`)
	deployDockerfileStep = regexp.MustCompile(`(?m)^\s*\d+\s*\|\s*>>>\s*(.+)$`)
	deployExitCode       = regexp.MustCompile(`exit code[: ]+([0-9]+)`)
	deployBuildError     = regexp.MustCompile(`(?m)^(?:#[0-9]+ ERROR: |error: failed to solve: )(.+)$`)
	deployShellSafe      = regexp.MustCompile(`^[a-zA-Z0-9_./:=@-]+$`)
)

func sanitizeDeployText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = deployTerminalEscape.ReplaceAllString(s, "")
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if (unicode.IsControl(r) && r != '\n') || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, s)
}

func boundedDeployText(s string, limit int) string {
	s = sanitizeDeployText(s)
	if len(s) <= limit {
		return s
	}
	// Dropping the invalid tail avoids cutting a UTF-8 code point in half.
	return strings.ToValidUTF8(s[:limit], "")
}

func deployEvidenceTail(message string) []string {
	lines := strings.Split(sanitizeDeployText(message), "\n")
	var tail []string
	bytes := 0
	for i := len(lines) - 1; i >= 0 && len(tail) < deployEvidenceLines && bytes < deployEvidenceBytes; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || PrivateFailureDetail.MatchString(line) {
			continue
		}
		line = boundedDeployText(line, deployEvidenceBytes-bytes)
		tail = append(tail, line)
		bytes += len(line) + 1
	}
	slices.Reverse(tail)
	return tail
}

// failureEvidence returns the newest event if it is a failure, its message with
// terminal controls removed, and that message minus infrastructure details
// (PrivateFailureDetail). Details are dropped line by line, so one URL in a
// build log cannot hide the rest of it. A newer non-failure never borrows an
// older failure's message.
func failureEvidence(raw json.RawMessage) (event Event, text, safe string) {
	event, ok := LatestEvent(raw, "")
	if !ok || (!strings.EqualFold(event.Status, "failed") && !strings.HasSuffix(event.Type, ".failed")) {
		return event, "", ""
	}
	var kept []string
	for _, line := range strings.Split(strings.ReplaceAll(event.Message, "\r\n", "\n"), "\n") {
		if !PrivateFailureDetail.MatchString(line) {
			kept = append(kept, line)
		}
	}
	return event, strings.TrimSpace(sanitizeDeployText(event.Message)), strings.TrimSpace(sanitizeDeployText(strings.Join(kept, "\n")))
}

// FailureDiagnostics explains why kind/name failed, judged from its events, or
// for code DEPLOY_TIMEOUT why it was not confirmed. started is when this
// invocation applied it; noBuildWithin is set when no build started within that
// long of the upload; redeploy builds the command that redeploys it.
func FailureDiagnostics(kind, name, code string, events json.RawMessage, started time.Time, noBuildWithin time.Duration, redeploy func(workspace string) []string) *Diagnostics {
	workspace := core.GetWorkspace()
	diagnostic := &Diagnostics{Code: code, Phase: "rollout", Cause: noSpecificCause, Next: Next{
		Message: "Inspect startup logs and check the application's listen address and port.",
		Command: []string{"bl", "logs", kind, name, "-w", workspace, "--period", "30m", "--utc"},
	}}
	if code == "DEPLOY_TIMEOUT" {
		diagnostic.Phase = "monitor"
		diagnostic.Cause = "Timed out waiting for this deployment's build and rollout; the resource may still be deploying."
		diagnostic.Next.Message = "Check the resource's status; a monitoring timeout is not a confirmed deployment failure."
		if noBuildWithin > 0 {
			diagnostic.Cause = fmt.Sprintf("No build started within %s of the upload. The platform may have dropped the update (for example because the previous rollout was still in flight) or be slow to start it. The previous revision is likely still serving; this deploy was not confirmed.", noBuildWithin)
			diagnostic.Next.Message = "Check the resource's status; this is not a confirmed failure. Redeploy once any rollout in progress has finished."
		}
		diagnostic.Next.Command = []string{"bl", "get", kind, name, "-w", workspace, "--watch"}
		return diagnostic
	}

	event, text, message := failureEvidence(events)
	if strings.Contains(event.Type, ".buildimage.") {
		diagnostic.Code, diagnostic.Phase = "BUILD_FAILED", "build"
		diagnostic.Next.Message = "Fix the build failure, then redeploy it from the directory you ran this in."
		diagnostic.Next.Command = redeploy(workspace)
		// The step, instruction keyword and exit code come from the full text, as
		// they are not infrastructure details; a failing command line that holds one
		// (a URL, say) is reduced to its keyword.
		diagnostic.Step = deployDockerfileLine.FindString(text)
		instruction := ""
		if step := deployDockerfileStep.FindStringSubmatch(text); len(step) > 1 {
			instruction = strings.TrimSpace(step[1])
			if PrivateFailureDetail.MatchString(instruction) {
				instruction = strings.Fields(instruction)[0]
			}
			diagnostic.Step = strings.TrimSpace(diagnostic.Step + " " + instruction)
		}
		if exit := deployExitCode.FindStringSubmatch(text); len(exit) > 1 && strings.HasPrefix(instruction, "RUN") {
			diagnostic.Cause = "Dockerfile RUN failed with exit code " + exit[1]
		} else if cause := deployBuildError.FindStringSubmatch(message); len(cause) > 1 {
			diagnostic.Cause = cause[1]
		} else if message != "" {
			diagnostic.Cause = strings.Split(message, "\n")[0]
		}
	} else {
		diagnostic.Code = "ROLLOUT_FAILED"
		if message != "" && !strings.EqualFold(strings.TrimSpace(message), "Deployment has failed") && !strings.EqualFold(strings.TrimSpace(message), "Deployment failed") {
			diagnostic.Cause = strings.Split(message, "\n")[0]
		}
	}
	if message != "" {
		diagnostic.Source = "resource.events"
		diagnostic.LogTail = deployEvidenceTail(message)
	}
	if diagnostic.Code == "ROLLOUT_FAILED" && diagnostic.Cause == noSpecificCause {
		// One page of logs, failure-only, within 5 seconds. A diagnostic error
		// must never hide the original rollout failure.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		logs, err := monitor.NewLogFetcher(core.GetClient(), workspace, kind, name, started.Add(-time.Minute), time.Now(), "", "", "", "").FetchLogsContext(ctx)
		cancel()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not read recent runtime evidence; use the next command to inspect logs.")
		} else if len(logs) > 0 {
			lines := make([]string, len(logs))
			for i, log := range logs {
				lines[i] = log.Message
			}
			if tail := deployEvidenceTail(strings.Join(lines, "\n")); len(tail) > 0 {
				diagnostic.Source, diagnostic.LogTail = "runtime.logs", tail
			}
		}
	}
	diagnostic.Cause = boundedDeployText(diagnostic.Cause, 1024)
	diagnostic.Step = boundedDeployText(diagnostic.Step, 1024)
	return diagnostic
}

// SubmittedNote is the one-line reminder shown when a non-interactive deploy
// returned after submission, so success is not read as "deployed".
func SubmittedNote(kind, name string) string {
	return fmt.Sprintf("Submitted, not finished. Follow it with `%s`, or add --wait next time.",
		deployCommandText([]string{"bl", "get", kind, name, "-w", core.GetWorkspace(), "--watch"}))
}

func deployCommandText(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		arg = sanitizeDeployText(arg)
		if deployShellSafe.MatchString(arg) {
			quoted[i] = arg
		} else {
			quoted[i] = "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
		}
	}
	return strings.Join(quoted, " ")
}

// Print writes the diagnostics of kind/name to stderr.
func (d *Diagnostics) Print(kind, name string) {
	title := "Deploy failed"
	if d.Code == "DEPLOY_TIMEOUT" {
		title = "Deploy not confirmed"
	}
	fmt.Fprintf(os.Stderr, "✗ %s: %s/%s (workspace %s)\nphase: %s\ncause: %s\n", title, sanitizeDeployText(kind), sanitizeDeployText(name), sanitizeDeployText(core.GetWorkspace()), d.Phase, d.Cause)
	if d.Step != "" {
		fmt.Fprintln(os.Stderr, "step:", d.Step)
	}
	if len(d.LogTail) > 0 {
		fmt.Fprintf(os.Stderr, "Evidence (%s, untrusted, at most 20 lines):\n", d.Source)
		for _, line := range d.LogTail {
			fmt.Fprintln(os.Stderr, "  |", line)
		}
	}
	fmt.Fprintf(os.Stderr, "next: %s Run `%s`.\n", d.Next.Message, deployCommandText(d.Next.Command))
}
