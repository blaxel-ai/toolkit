package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/customdomain"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	core.RegisterCommand("custom-domain", func() *cobra.Command {
		return CustomDomainCmd()
	})
}

var customDomainResource = core.Resource{
	Kind:     "CustomDomain",
	Plural:   "customdomains",
	Singular: "customdomain",
	Fields: []core.Field{
		{Key: "NAME", Value: "name"},
		{Key: "TYPE", Value: "spec.domainType"},
		{Key: "STATUS", Value: "spec.status"},
		{Key: "VALIDATION", Value: "spec.validationMethod"},
		{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
	},
}

func CustomDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "custom-domain",
		Aliases: []string{"custom-domains", "cd"},
		Short:   "Manage custom domains",
		Long: `Manage the custom domains of the current workspace.

A custom domain serves previews or applications under your own hostname. After
creating it, publish the DNS records shown by the records command, then verify it.

` + "```" + `
  bl custom-domain list                      List custom domains
  bl custom-domain get <name>                Get a custom domain
  bl custom-domain create <name> --type ...  Create a custom domain
  bl custom-domain records <name>            Show the DNS records to publish
  bl custom-domain verify <name>             Check the DNS records and activate the domain
  bl custom-domain delete <name>             Delete a custom domain
` + "```",
		Example: `  # Create an application domain with a www alternative name
  bl custom-domain create example.com --type applications --san www.example.com

  # Show the DNS records to publish, then verify
  bl custom-domain records example.com
  bl custom-domain verify example.com`,
	}

	cmd.AddCommand(CustomDomainListCmd())
	cmd.AddCommand(CustomDomainGetCmd())
	cmd.AddCommand(CustomDomainCreateCmd())
	cmd.AddCommand(CustomDomainRecordsCmd())
	cmd.AddCommand(CustomDomainVerifyCmd())
	cmd.AddCommand(CustomDomainDeleteCmd())
	return cmd
}

func CustomDomainListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List custom domains",
		Example: `  bl custom-domain list
  bl custom-domain list -o json`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			domains, err := customdomain.List(context.Background(), core.GetClient())
			if err != nil {
				exitCustomDomainError("List custom domains", err)
			}
			core.Output(customDomainResource, domains, core.GetOutputFormat())
		},
	}
}

func CustomDomainGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <name>",
		Short: "Get a custom domain",
		Example: `  bl custom-domain get example.com
  bl custom-domain get example.com -o yaml`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			domain, err := customdomain.Get(context.Background(), core.GetClient(), args[0])
			if err != nil {
				exitCustomDomainError("Get custom domain", err)
			}
			core.Output(customDomainResource, []any{domain}, core.GetOutputFormat())
		},
	}
}

func CustomDomainCreateCmd() *cobra.Command {
	var domainType, validationMethod string
	var sans []string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a custom domain",
		Long: `Create a custom domain in the current workspace.

--type previews serves sandbox previews under the domain (use a wildcard name
such as *.preview.example.com); --type applications serves applications.

--validation-method chooses how the TLS certificate is validated. dns (the API
default when the flag is omitted) asks you to publish certificate validation
records. http needs no validation record once the domain points to Blaxel; it
is only available for applications domains without wildcard names. The method
is set at creation: later updates keep the existing method.`,
		Example: `  # Application domain on an apex with www, DNS validation (default)
  bl custom-domain create example.com --type applications --san www.example.com

  # Application domain validated over HTTP
  bl custom-domain create app.example.com --type applications --validation-method http

  # Wildcard previews domain
  bl custom-domain create "*.preview.example.com" --type previews`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			req, err := customdomain.BuildCreateRequest(customdomain.CreateOptions{
				Name:                    args[0],
				DomainType:              domainType,
				SubjectAlternativeNames: sans,
				ValidationMethod:        validationMethod,
			})
			if err != nil {
				err = core.MarkExpectedError(err, core.CLIErrorValidation)
				core.PrintError("Create custom domain", err)
				core.ExitWithError(err)
			}

			domain, err := customdomain.Create(context.Background(), core.GetClient(), req)
			if err != nil {
				exitCustomDomainError("Create custom domain", err)
			}

			format := core.GetOutputFormat()
			if format != "" && format != "table" {
				core.Output(customDomainResource, []any{domain}, format)
				return
			}
			name := domainName(domain, req.Metadata.Name)
			core.PrintSuccess(fmt.Sprintf("Custom domain '%s' created", name))
			printRecords(cmd.OutOrStdout(), customdomain.Records(domain))
			core.PrintInfoWithCommand("Publish these records, then run", fmt.Sprintf("bl custom-domain verify %s", name))
		},
	}

	cmd.Flags().StringVar(&domainType, "type", "", "Domain type: previews or applications")
	cmd.Flags().StringSliceVar(&sans, "san", nil, "Subject alternative name to add to the certificate (repeatable, applications only)")
	cmd.Flags().StringVar(&validationMethod, "validation-method", "", "Certificate validation method: dns or http (API default: dns)")
	_ = cmd.MarkFlagRequired("type")
	return cmd
}

func CustomDomainRecordsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "records <name>",
		Short: "Show the DNS records to publish for a custom domain",
		Long: `Show the DNS records to publish at your DNS provider for a custom domain.

Routing records point the domain to Blaxel. An apex name (example.com) cannot
carry a CNAME, so its routing record is shown as ALIAS/ANAME: use your
provider's ALIAS, ANAME or CNAME flattening record. Certificate validation
records are only needed with the dns validation method.`,
		Example: `  bl custom-domain records example.com
  bl custom-domain records example.com -o json`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			domain, err := customdomain.Get(context.Background(), core.GetClient(), args[0])
			if err != nil {
				exitCustomDomainError("Get custom domain records", err)
			}
			records := customdomain.Records(domain)
			switch core.GetOutputFormat() {
			case "json":
				out, _ := json.MarshalIndent(records, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
			case "yaml", "pretty":
				out, _ := yaml.Marshal(records)
				fmt.Fprint(cmd.OutOrStdout(), string(out))
			default:
				printRecords(cmd.OutOrStdout(), records)
			}
		},
	}
}

func CustomDomainVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "verify <name>",
		Short:   "Check the DNS records of a custom domain and activate it",
		Example: `  bl custom-domain verify example.com`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			domain, err := customdomain.Verify(context.Background(), core.GetClient(), args[0])
			if err != nil {
				exitCustomDomainError("Verify custom domain", err)
			}
			format := core.GetOutputFormat()
			if format != "" && format != "table" {
				core.Output(customDomainResource, []any{domain}, format)
				return
			}
			spec, _ := domain["spec"].(map[string]any)
			status, _ := spec["status"].(string)
			if status == "verified" {
				core.PrintSuccess(fmt.Sprintf("Custom domain '%s' is verified", args[0]))
				return
			}
			msg := fmt.Sprintf("Custom domain '%s' is %s", args[0], status)
			if reason, _ := spec["verificationError"].(string); reason != "" {
				msg += ": " + reason
			}
			core.PrintWarning(msg)
		},
	}
}

func CustomDomainDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a custom domain",
		Example: `  bl custom-domain delete example.com`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := customdomain.Delete(context.Background(), core.GetClient(), args[0]); err != nil {
				exitCustomDomainError("Delete custom domain", err)
			}
			core.PrintSuccess(fmt.Sprintf("Custom domain '%s' deleted", args[0]))
		},
	}
}

func exitCustomDomainError(operation string, err error) {
	err = customdomain.DescribeError(err)
	core.PrintError(operation, err)
	core.ExitWithError(err)
}

func domainName(domain map[string]any, fallback string) string {
	if metadata, ok := domain["metadata"].(map[string]any); ok {
		if name, _ := metadata["name"].(string); name != "" {
			return name
		}
	}
	return fallback
}

func printRecords(w io.Writer, records []customdomain.Record) {
	if len(records) == 0 {
		core.PrintInfo("No DNS records to publish yet")
		return
	}
	tw := table.NewWriter()
	tw.AppendHeader(table.Row{"TYPE", "NAME", "VALUE", "PURPOSE"})
	for _, r := range records {
		tw.AppendRow(table.Row{r.Type, r.Name, r.Value, r.Purpose})
	}
	_, _ = fmt.Fprintln(w, tw.Render())
	for _, r := range records {
		if r.Type == customdomain.RecordTypeAlias {
			core.PrintInfo("ALIAS/ANAME: an apex name cannot carry a CNAME, use your DNS provider's ALIAS, ANAME or CNAME flattening record")
			break
		}
	}
}
