// Command webtoken is magpie's panel access key, supplied to the DSM identity
// gateway as a request header after a DSM administrator signs in.
//
// magpie serve protects its web panel with MAGPIE_WEB_KEY: a request needs the
// key in the `magpie_web_<port>` cookie (or the link's ?k=). The gateway runs
// this adapter as the package account for each admitted request and replaces
// the client's cookies with the returned one, so the browser never receives
// the key and the panel needs no second App password.
//
// The adapter is a fixed argument program: the cookie name comes from argv and
// the key from the daemon environment. It mints nothing and stores nothing.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// keyEnv is the variable `magpie serve` reads the panel's key from.
const keyEnv = "MAGPIE_WEB_KEY"

// minKeyLen is the shortest key magpie accepts (gui.CheckWebKey).
const minKeyLen = 16

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: webtoken <cookie-name>  (for example magpie_web_3430)\n")
		os.Exit(2)
	}
	out, err := credentials(os.Args[1], os.Getenv(keyEnv))
	if err != nil {
		fmt.Fprintf(os.Stderr, "webtoken: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(out)
}

// credentials is the gateway adapter reply for name and key: the only header
// the gateway sets upstream is the panel's own cookie. magpie's key alphabet
// (letters, digits and - . _ ~) is already a valid cookie value.
func credentials(name, key string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("the cookie name must not be empty")
	}
	if len(key) < minKeyLen {
		// Refuse rather than send a cookie magpie would reject: a wrong panel
		// key would look like a broken sign-in.
		return nil, fmt.Errorf("%s is unset or shorter than %d characters", keyEnv, minKeyLen)
	}
	out, err := json.Marshal(map[string]map[string][]string{
		"headers": {"Cookie": {name + "=" + key}},
	})
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
