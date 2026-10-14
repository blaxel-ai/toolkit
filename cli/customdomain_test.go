package cli

import (
	"bytes"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/customdomain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomDomainCmdStructure(t *testing.T) {
	cmd := CustomDomainCmd()
	assert.Equal(t, "custom-domain", cmd.Use)
	assert.Equal(t, []string{"custom-domains", "cd"}, cmd.Aliases)

	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.ElementsMatch(t, []string{"list", "get", "create", "records", "verify", "delete"}, names)
}

func TestCustomDomainCreateFlags(t *testing.T) {
	cmd := CustomDomainCreateCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--type", "applications", "--san", "www.example.com", "--san", "api.example.com", "--validation-method", "http"}))

	domainType, _ := cmd.Flags().GetString("type")
	sans, _ := cmd.Flags().GetStringSlice("san")
	method, _ := cmd.Flags().GetString("validation-method")
	assert.Equal(t, "applications", domainType)
	assert.Equal(t, []string{"www.example.com", "api.example.com"}, sans)
	assert.Equal(t, "http", method)

	fresh := CustomDomainCreateCmd()
	require.NoError(t, fresh.ParseFlags([]string{"--type", "previews"}))
	assert.False(t, fresh.Flags().Changed("validation-method"))
	method, _ = fresh.Flags().GetString("validation-method")
	assert.Empty(t, method, "validation method must stay empty so it is not sent")
	assert.NotNil(t, fresh.Args)
}

func TestPrintRecordsWritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	printRecords(&buf, []customdomain.Record{{Type: customdomain.RecordTypeAlias, Name: "example.com", Value: "apps.example.net", Purpose: customdomain.PurposeRouting}})
	out := buf.String()
	assert.Contains(t, out, "ALIAS/ANAME")
	assert.Contains(t, out, "example.com")
	assert.NotContains(t, out, "CNAME")
}
