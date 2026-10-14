package prompt

import "os"

// inputWaiting reports whether the console in may have input that was not
// read yet. It reports true: an answer at a Windows console ends with CR LF,
// so the LF that readLine drops first after a hidden answer is never the
// Enter of an empty answer.
func inputWaiting(*os.File) bool {
	return true
}
