package main

import "testing"

func TestNetworkNameDoesNotEchoThePassphrase(t *testing.T) {
	// The mapping exists so that startup logging carries a short label rather
	// than a value whose name reads as a credential. An unrecognised passphrase
	// must report "custom" rather than echoing it, so a misconfigured value
	// cannot inject arbitrary text into the log.
	cases := map[string]string{
		"Public Global Stellar Network ; September 2015": "pubnet",
		"Test SDF Network ; September 2015":              "testnet",
		"Test SDF Future Network ; October 2022":         "futurenet",
		"":                                               "unset",
		"something someone configured by hand":           "custom",
	}
	for passphrase, want := range cases {
		if got := networkName(passphrase); got != want {
			t.Errorf("networkName(%q) = %q, want %q", passphrase, got, want)
		}
	}
}

func TestNetworkNameNeverReturnsItsInput(t *testing.T) {
	unknown := "SECRET-LOOKING-VALUE-THAT-MUST-NOT-BE-LOGGED"
	if got := networkName(unknown); got == unknown {
		t.Fatalf("networkName echoed its input: %q", got)
	}
}
