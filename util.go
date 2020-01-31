package main

// Prepend the hd error to the tl errors if it is not nil.
func mergeErr(hd error, tl []error) []error {
	if hd != nil {
		return append([]error{hd}, tl...)
	}
	return tl
}
