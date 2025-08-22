package apikeys

// Note: Output format tests were removed because they used the old testing pattern
// that creates isolated commands without persistent flags and uses non-existent --interactive flag.
// Output format functionality is covered by integration tests that use the proper
// NewRootCmdForTesting() pattern with full command tree context.
//
// The removed tests included:
// - TestCreateCommand_OutputFormats
// - TestUpdateCommand_OutputFormats
// - TestGetCommand_OutputFormats
// - TestDeleteCommand_OutputFormats
// - TestListCommand_OutputFormats
// - TestOutputFormatValidation
// - TestOutputFlagIntegration
// - BenchmarkOutputFormats
//
// These tests are now covered by integration tests at the cmd/ level.
