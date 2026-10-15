package version

// Version is the CLI version, set at build time
var Version = "dev"

// BuildTime is the time of the commit the CLI was built from, in RFC 3339,
// set at build time
var BuildTime = "unknown"

// GitCommit is the git commit hash, set at build time
var GitCommit = "unknown"
