// Command medkeygen creates an Ed25519 .medkey dongle file for a doctor.
//
// The private seed is written once to the target path with 0600 permissions.
// The file is meant to be copied to a USB flash drive and opened by the browser
// through the File System Access API.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"medvault/internal/crypto"
)

func main() {
	username := flag.String("username", "", "doctor username the dongle belongs to")
	keyID := flag.String("key-id", "", "human readable dongle identifier, for example DONGLE-2026-0001")
	out := flag.String("out", "", "output path, for example ./medkeys/dr_andi.medkey")
	flag.Parse()

	if *username == "" || *keyID == "" || *out == "" {
		log.Fatal("username, key-id, and out are required")
	}

	key, err := crypto.GenerateMedKey(*keyID, *username)
	if err != nil {
		log.Fatalf("generate dongle: %v", err)
	}
	if err := key.Validate(); err != nil {
		log.Fatalf("self check: %v", err)
	}
	if err := crypto.WriteMedKey(*out, key); err != nil {
		log.Fatalf("write dongle: %v", err)
	}

	fmt.Printf("medkey created\n")
	fmt.Printf("  path       %s\n", *out)
	fmt.Printf("  username   %s\n", key.Username)
	fmt.Printf("  key id     %s\n", key.KeyID)
	fmt.Printf("  public key %s\n", key.PublicKeyHex)
	fmt.Printf("keep the private seed on the USB drive, never in the repository\n")

	if info, err := os.Stat(*out); err == nil {
		fmt.Printf("  mode       %o\n", info.Mode().Perm())
	}
}
