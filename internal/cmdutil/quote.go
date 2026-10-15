package cmdutil

import (
	"runtime"
	"strings"
	"unicode"
)

// ShellQuote returns value as a word a user can paste into a command line of
// a POSIX shell, such as bash or zsh: value itself when it holds only
// characters no such shell treats specially, otherwise value in single
// quotes, where each single quote of value ends the quoting, is escaped with
// a backslash and starts it again. A value that starts with =, which zsh
// expands to the path of a command, is quoted too.
func ShellQuote(value string) string {
	if value != "" && strings.Trim(value, shellSafe) == "" && !strings.HasPrefix(value, "=") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// QuotePath returns path as an argument a user can paste into a command line
// of the system's shell: on Windows, whose cmd.exe passes single quotes on
// as part of the argument, path in double quotes when it holds a character
// other than a letter, a digit or one of windowsPathSafe, such as a space,
// ', &, ^ or $, and path itself otherwise; elsewhere, path as ShellQuote
// quotes it.
func QuotePath(path string) string {
	return quotePath(runtime.GOOS, path)
}

// windowsPathSafe holds the characters besides letters and digits a Windows
// path may hold unquoted in cmd.exe and PowerShell.
const windowsPathSafe = `_-.:\/`

// quotePath is QuotePath on the operating system goos.
func quotePath(goos, path string) string {
	if goos != "windows" {
		return ShellQuote(path)
	}
	for _, r := range path {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune(windowsPathSafe, r) {
			return `"` + path + `"`
		}
	}
	return path
}

// shellSafe holds the characters a shell word may hold unquoted.
const shellSafe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@%+=:,./_-"
