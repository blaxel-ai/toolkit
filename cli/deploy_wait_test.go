package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/server"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDeployWaitFlagIsOptIn(t *testing.T) {
	flags := DeployCmd().Flags()
	require.Equal(t, "false", flags.Lookup("wait").DefValue, "waiting must stay opt-in")
	require.NotNil(t, flags.Lookup("timeout"))

	commands := func() []server.PackageCommand {
		return []server.PackageCommand{{Args: []string{"deploy"}}, {Args: []string{"deploy", "-s", "A=b"}}}
	}
	plain := commands()
	forwardWaitFlags(plain, false, "5m")
	require.Equal(t, commands(), plain, "without --wait, -R packages deploy exactly as before")
	waiting := commands()
	forwardWaitFlags(waiting, true, "5m")
	require.Equal(t, []string{"deploy", "-s", "A=b", "--wait", "--timeout", "5m"}, waiting[1].Args)
}

// deployWithArchive builds a Deployment whose apply returns an upload URL, so
// the whole submit-and-upload path runs against the test server.
func deployWithArchive(t *testing.T) *Deployment {
	t.Helper()
	d := &Deployment{cwd: t.TempDir(), name: "test", timeout: time.Second, blaxelDeployments: []core.Result{{Kind: "Agent", Metadata: map[string]any{"name": "test", "labels": map[string]any{"x-blaxel-auto-generated": "true"}}, Spec: map[string]any{}}}}
	archive, err := os.CreateTemp(t.TempDir(), "archive.zip")
	require.NoError(t, err)
	_, err = archive.WriteString("archive")
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	d.archive = archive
	return d
}

func TestDeployFlagMatrix(t *testing.T) {
	deployedAt := func(revision string) string {
		return fmt.Sprintf(`{"status":"DEPLOYED","events":[{"status":"DEPLOYED","revision":%q,"time":"2026-10-06T00:00:00Z"}]}`, revision)
	}
	hint := "Submitted, not finished. Follow it with `bl get agent test -w test-workspace --watch`, or add --wait next time"
	for _, wait := range []bool{false, true} {
		for _, format := range []string{"pretty", "json", "yaml"} {
			t.Run(fmt.Sprintf("wait=%v/%s", wait, format), func(t *testing.T) {
				core.ResetConfig()
				core.SetConfigType("agent")
				var applies, uploads, reads atomic.Int32
				deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case r.URL.Path == "/upload":
						uploads.Add(1)
						_, _ = io.Copy(io.Discard, r.Body)
					case r.Method == http.MethodGet:
						reads.Add(1)
						revision := "r0"
						if uploads.Load() > 0 {
							revision = "r1" // this build's rollout
						}
						_, _ = w.Write([]byte(deployedAt(revision)))
					default:
						applies.Add(1)
						w.Header().Set("X-Blaxel-Upload-Url", "http://"+r.Host+"/upload")
						_, _ = w.Write([]byte(`{"metadata":{"name":"test","url":"https://example.test"}}`))
					}
				})
				d := deployWithArchive(t)
				require.NoError(t, d.applyNonInteractive(wait))
				require.EqualValues(t, 1, applies.Load())
				require.EqualValues(t, 1, uploads.Load())
				if wait {
					require.EqualValues(t, 2, reads.Load(), "baseline before apply, then wait for this build")
					require.Len(t, d.observations, 1)
				} else {
					require.Zero(t, reads.Load(), "a plain submission reads nothing before returning")
					require.Empty(t, d.observations)
				}

				if format == "pretty" {
					output := captureDeployStdout(t, d.Ready)
					require.Contains(t, output, "Deployment applied successfully")
					if wait {
						require.NotContains(t, output, "Submitted, not finished")
					} else {
						require.Contains(t, output, hint)
						require.Equal(t, 1, strings.Count(output, "Submitted, not finished"), "exactly one hint line")
					}
					return
				}
				output := captureDeployStdout(t, func() { d.printStructuredOutput(format, time.Now(), false, nil) })
				var result struct {
					Success   bool             `json:"success" yaml:"success"`
					Resources []map[string]any `json:"resources" yaml:"resources"`
				}
				if format == "json" {
					require.NoError(t, json.Unmarshal([]byte(output), &result))
				} else {
					require.NoError(t, yaml.Unmarshal([]byte(output), &result))
				}
				require.True(t, result.Success)
				resource := result.Resources[0]
				require.NotContains(t, resource, "diagnostics")
				if wait {
					require.NotContains(t, resource, "note")
					require.Equal(t, "DEPLOYED", resource["status"])
				} else {
					require.Contains(t, resource["note"], hint)
				}
			})
		}
	}
}

func TestDeployFailedSubmissionIsAPlainErrorWithOrWithoutWait(t *testing.T) {
	core.ResetConfig()
	core.SetConfigType("agent")
	var reads atomic.Int32
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) { // every request is refused
		if r.Method == http.MethodGet {
			reads.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Forbidden"}`))
	})
	for _, wait := range []bool{false, true} {
		d := Deployment{name: "test", cwd: t.TempDir(), blaxelDeployments: []core.Result{{Kind: "Agent", Metadata: map[string]any{"name": "test"}, Spec: map[string]any{}}}}
		err := d.applyNonInteractive(wait)
		require.ErrorContains(t, err, "failed to apply Agent/test")
		require.Empty(t, d.observations, "nothing to wait for after a failed submission")
		require.False(t, d.printFailureDiagnostics(), "the plain PrintError path is used")
		output := captureDeployStdout(t, func() { d.printStructuredOutput("json", time.Now(), true, err) })
		require.Contains(t, output, `"status": "FAILED"`)
		require.NotContains(t, output, "diagnostics")
		require.NotContains(t, output, `"note"`)
	}
	require.EqualValues(t, 1, reads.Load(), "only the wait run read the baseline")
}

func TestDeployWaitDoesNotInventBuildWhenApplyReturnsNoUpload(t *testing.T) {
	core.ResetConfig()
	core.SetConfigType("agent")
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"status":"DEPLOYED","events":[{"status":"DEPLOYED","revision":"r0","time":"2026-10-01T00:00:00Z"}]}`))
		} else {
			_, _ = w.Write([]byte(`{"metadata":{"name":"test"}}`))
		}
	})
	d := Deployment{name: "test", cwd: t.TempDir(), timeout: time.Second, blaxelDeployments: []core.Result{{Kind: "Agent", Metadata: map[string]any{"name": "test", "labels": map[string]any{"x-blaxel-auto-generated": "true"}}, Spec: map[string]any{}}}}
	require.NoError(t, d.applyNonInteractive(true))
	require.False(t, d.observations[0].autoGenerated, "no archive was submitted; use the existing non-built completion rule")
	require.Equal(t, "DEPLOYED", d.observations[0].rollout.Status)
}

func TestDeployWaitEndsOnStatusAloneWhenThereAreNoEvents(t *testing.T) {
	for _, status := range []string{"FAILED", "DEACTIVATED"} {
		t.Run(status, func(t *testing.T) {
			deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"status":%q}`, status)
			})
			d := Deployment{cwd: t.TempDir(), observations: []deployObservation{{kind: "agent", name: "test", started: time.Now(), initialStatus: "DEPLOYED"}}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.Error(t, d.waitForRollouts(ctx, time.Millisecond))
			require.Equal(t, "ROLLOUT_FAILED", d.observations[0].diagnostics.Code)
		})
	}
}

// ev is one platform event, offset from base.
func ev(base time.Time, offset time.Duration, kind, status, revision, message string) failureEvent {
	return failureEvent{Type: "ai.blaxel.controlplane." + kind, Status: status, Revision: revision, Message: message, Time: base.Add(offset).UTC().Format(time.RFC3339Nano)}
}

func eventsJSON(t *testing.T, events []failureEvent) string {
	t.Helper()
	data, err := json.Marshal(events)
	require.NoError(t, err)
	return string(data)
}

// redeploy returns the history of an agent deployed as r0, that history after a
// redeploy was accepted and answered by a reused image build (the platform then
// re-applies the given revision), and once that revision is rolled out and finished.
func redeploy(started time.Time, revision string) (baseline, reused, finished []failureEvent) {
	baseline = []failureEvent{
		ev(started, -time.Hour, "buildimage.created", "UPLOADED", "", "Starting build image"),
		ev(started, -time.Hour+time.Second, "buildimage.succeeded", "BUILT", "", "Build image succeeded"),
		ev(started, -time.Hour+2*time.Second, "deployment.ready", "DEPLOYED", "r0", "Deployment ready on a cluster"),
		ev(started, -time.Hour+3*time.Second, "deployment.succeeded", "DEPLOYED", "r0", "Deployments successfully done"),
	}
	reused = append(append([]failureEvent{}, baseline...),
		ev(started, time.Second, "api.update", "UPDATING", "", "Update agent"),
		ev(started, 4*time.Second, "buildimage.succeeded", "BUILT", "", "Reused an identical image build"),
		ev(started, 4400*time.Millisecond, "api.update", "UPDATED", revision, "Update deployment"),
	)
	finished = append(append([]failureEvent{}, reused...),
		ev(started, 6*time.Second, "deployment.ready", "DEPLOYED", revision, "Deployment ready on a cluster"),
		ev(started, 12*time.Second, "deployment.succeeded", "DEPLOYED", revision, "Deployments successfully done"))
	return baseline, reused, finished
}

func unchangedObservation(t *testing.T, started time.Time, baseline []failureEvent) deployObservation {
	t.Helper()
	o := deployObservation{kind: "agent", name: "test", started: started, autoGenerated: true, baseline: rolloutBaseline{revision: "r0", deployedRevision: "r0", known: true}, baselineEvents: map[failureEvent]struct{}{}}
	for _, e := range baseline {
		o.baselineEvents[e] = struct{}{}
	}
	return o
}

func TestDeployWaitReportsUnchangedRedeployAsSuccess(t *testing.T) {
	core.SetConfigType("agent")
	for _, tc := range []struct {
		name, revision string
		unchanged      bool
	}{
		{"unchanged source re-applies the revision", "r0", true},
		{"rollback to a cached build creates a new revision", "r1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := time.Now()
			baseline, reused, finished := redeploy(started, tc.revision)
			states := [][]failureEvent{
				reused[:len(baseline)+1], // update accepted, source not yet recognized
				reused,                   // reused, but nothing has deployed since: must not finish on the old DEPLOYED
				finished[:len(reused)+1], // rolling out: DEPLOYED is reported, but the rollout is not finished
				finished,
			}
			var reads atomic.Int32
			deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				n := min(int(reads.Add(1)), len(states))
				status := "DEPLOYED"
				if n == 1 {
					status = "DEPLOYING"
				}
				_, _ = fmt.Fprintf(w, `{"status":%q,"events":%s}`, status, eventsJSON(t, states[n-1]))
			})
			d := Deployment{name: "test", observations: []deployObservation{unchangedObservation(t, started, baseline)}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, d.waitForRollouts(ctx, time.Millisecond))
			require.EqualValues(t, len(states), reads.Load(), "waits for the platform's final event")
			o := d.observations[0]
			require.False(t, o.reusedAt.IsZero())
			require.Nil(t, o.diagnostics)
			if !tc.unchanged {
				require.Empty(t, o.notice, "a reused build with a new revision is not an unchanged deploy")
				return
			}
			require.Equal(t, "No new revision: the source is unchanged, so the platform reused the identical image build and re-applied revision r0 of agent/test.", o.notice)
			// Pretty output carries the notice and no "submitted" hint; structured output a note.
			d.wait = true
			pretty := captureDeployStdout(t, d.Ready)
			require.Contains(t, pretty, "No new revision")
			require.NotContains(t, pretty, "Submitted, not finished")
			var result struct {
				Resources []struct{ Note string } `json:"resources"`
			}
			require.NoError(t, json.Unmarshal([]byte(captureDeployStdout(t, func() { d.printStructuredOutput("json", time.Now(), false, nil) })), &result))
			require.Equal(t, o.notice, result.Resources[0].Note)
		})
	}
}

func TestDeployWaitStopsHoldingForTheFinalEventAfterTheGrace(t *testing.T) {
	core.SetConfigType("agent")
	original := deployFinalGrace
	deployFinalGrace = 50 * time.Millisecond
	t.Cleanup(func() { deployFinalGrace = original })
	started := time.Now()
	baseline, reused, finished := redeploy(started, "r0")
	rolling := finished[:len(reused)+1] // DEPLOYED, but "deployment.succeeded" never arrives
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"DEPLOYED","events":%s}`, eventsJSON(t, rolling))
	})
	d := Deployment{name: "test", observations: []deployObservation{unchangedObservation(t, started, baseline)}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	begin := time.Now()
	require.NoError(t, d.waitForRollouts(ctx, 10*time.Millisecond))
	require.GreaterOrEqual(t, time.Since(begin), deployFinalGrace, "held for the final event first")
	require.Less(t, time.Since(begin), 2*time.Second)
}

func TestDeployWaitReportsFailureOfReappliedRevision(t *testing.T) {
	core.SetConfigType("agent")
	started := time.Now()
	baseline, reused, _ := redeploy(started, "r0")
	failed := append(append([]failureEvent{}, reused...), ev(started, 7*time.Second, "deployment.failed", "FAILED", "r0", "Container exited with code 1"))
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":%s}`, eventsJSON(t, failed))
	})
	d := Deployment{name: "test", cwd: t.TempDir(), observations: []deployObservation{unchangedObservation(t, started, baseline)}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Error(t, d.waitForRollouts(ctx, time.Millisecond))
	require.Equal(t, "ROLLOUT_FAILED", d.observations[0].diagnostics.Code)
	require.Empty(t, d.observations[0].notice)
}

func TestDeployWaitStopsWhenNoBuildFollowsTheUpload(t *testing.T) {
	core.SetConfigType("agent")
	original := deployNoBuildGrace
	deployNoBuildGrace = 50 * time.Millisecond
	t.Cleanup(func() { deployNoBuildGrace = original })
	started := time.Now()
	baseline, _, _ := redeploy(started, "r0")
	// The update was accepted but no build ever started; the previous rollout's
	// events keep arriving. They are new since the baseline but not build events.
	dropped := append(append([]failureEvent{}, baseline...),
		ev(started, time.Second, "api.update", "UPDATING", "", "Update agent"),
		ev(started, 2*time.Second, "deployment.succeeded", "DEPLOYED", "r0", "Deployments successfully done"))
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"DEPLOYED","events":%s}`, eventsJSON(t, dropped))
	})
	d := Deployment{name: "test", cwd: t.TempDir(), observations: []deployObservation{unchangedObservation(t, started, baseline)}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	begin := time.Now()
	err := d.waitForRollouts(ctx, 10*time.Millisecond)
	require.Error(t, err)
	require.Less(t, time.Since(begin), 10*time.Second, "bounded by the grace period, not by --timeout")
	require.True(t, core.IsExpectedCLIError(err))
	o := d.observations[0]
	require.True(t, o.noBuild)
	require.Equal(t, "DEPLOY_TIMEOUT", o.diagnostics.Code)
	require.Contains(t, o.diagnostics.Cause, "No build started within 50ms of the upload")
	require.Contains(t, o.diagnostics.Cause, "not confirmed")
	require.Equal(t, "DEPLOYED", o.rollout.Status, "the last observed status is kept, never relabelled")
	require.Empty(t, o.notice, "an unconfirmed deploy is never reported as success")
	require.Equal(t, []string{"bl", "get", "agent", "test", "-w", "test-workspace", "--watch"}, o.diagnostics.Next.Command)
}

func TestDeployWaitJudgesOnlyWhatItCanTell(t *testing.T) {
	started := time.Now()
	baseline, _, _ := redeploy(started, "r0")
	build := ev(started, time.Second, "buildimage.created", "UPLOADED", "", "Starting build image")
	reuse := ev(started, time.Second, "buildimage.succeeded", "BUILT", "", "Reused an identical image build")
	with := func(extra ...failureEvent) []failureEvent {
		return append(append([]failureEvent{}, baseline...), extra...)
	}
	cases := []struct {
		name            string
		history, events []failureEvent // before the apply, and now
		mutate          func(*deployObservation)
		notPicked       bool // buildNotPickedUp once the grace period has passed
		reusedSeen      bool // observeReusedBuild
	}{
		{"nothing new in a pipeline that builds", baseline, baseline, nil, true, false},
		{"a build started", baseline, with(build), nil, false, false},
		{"a build was reused", baseline, with(reuse), nil, false, true},
		{"no source build was submitted", baseline, baseline, func(o *deployObservation) { o.autoGenerated = false }, false, false},
		{"no readable baseline", baseline, with(reuse), func(o *deployObservation) { o.baselineEvents = nil }, false, false},
		{"a reuse from an earlier deploy is in the baseline", with(reuse), with(reuse), nil, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := unchangedObservation(t, started, tc.history)
			if tc.mutate != nil {
				tc.mutate(&o)
			}
			o.rollout = resourceRollout{Events: json.RawMessage(eventsJSON(t, tc.events))}
			require.Equal(t, tc.notPicked, o.buildNotPickedUp(deployNoBuildGrace))
			require.False(t, o.buildNotPickedUp(deployNoBuildGrace-time.Second), "within the grace period")
			o.observeReusedBuild()
			require.Equal(t, tc.reusedSeen, !o.reusedAt.IsZero())
		})
	}
}
