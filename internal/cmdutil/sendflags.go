package cmdutil

// The help of the flags messages send and smtp send share, so each flag reads
// the same in both. A command adds what applies to it alone, such as when a
// flag is required.
const (
	FromUsage          = "Sender email address"
	ToUsage            = "Recipient email address (can be used multiple times)"
	SubjectUsage       = "Email subject"
	TextUsage          = "Plain text content"
	HTMLUsage          = "HTML content"
	TextFileUsage      = "File with the plain text content"
	HTMLFileUsage      = "File with the HTML content"
	HeaderUsage        = `Custom header in 'Name: value' format, each name once in any case, in double quotes when the value holds a comma: '"Name: a, b"'`
	AttachUsage        = "File to attach"
	SandboxUsage       = "Send in sandbox mode, which delivers nothing"
	SandboxResultUsage = "Sandbox result simulation: %s (needs --sandbox)"
	TagUsage           = "Message tag (can be used multiple times)"
	TrackOpensUsage    = "Track opens (--track-opens=false turns it off); the account's setting applies when omitted"
	TrackClicksUsage   = "Track clicks (--track-clicks=false turns it off); the account's setting applies when omitted"
)
