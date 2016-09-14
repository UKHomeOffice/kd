package main

import "os"

// isAnyEnvSet sets if any of given environment vars is set and returns a boolean
func isAnyEnvSet(vars ...string) bool {
	var r bool
	for _, v := range vars {
		_, r := os.LookupEnv(v)
		if r {
			return r
		}
	}
	return r
}
