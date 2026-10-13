package main

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rawBaseOfHome is where each image home's git mirror serves install.sh from: a fresh install
// that pulls from one home must fetch its files from the same side.
var rawBaseOfHome = map[string]string{
	"ghcr.io/chetto1983":                  "raw.githubusercontent.com/chetto1983/Aura",
	"registry.gitlab.com/chetto1983/aura": "gitlab.com/Chetto1983/aura/-/raw",
}

// imageHomes reads the updater's IMAGE_HOME and IMAGE_HOMES, the one place the image homes are
// declared, and returns the current home and every other one.
func imageHomes(t *testing.T, root string) (string, []string) {
	t.Helper()
	updater := readProjectFile(t, root, "deploy/aura-image-update.sh")
	home := regexp.MustCompile(`(?m)^IMAGE_HOME=(\S+)$`).FindStringSubmatch(updater)
	homes := regexp.MustCompile(`(?m)^IMAGE_HOMES=\(([^)]*)\)$`).FindStringSubmatch(updater)
	if home == nil || homes == nil {
		t.Fatal("deploy/aura-image-update.sh declares no IMAGE_HOME or IMAGE_HOMES")
	}
	var others []string
	declared := false
	for h := range strings.FieldsSeq(homes[1]) {
		if h == home[1] {
			declared = true
			continue
		}
		if rawBaseOfHome[h] == "" {
			t.Fatalf("image home %s has no raw base in rawBaseOfHome", h)
		}
		others = append(others, h)
	}
	if !declared {
		t.Fatalf("IMAGE_HOME %s is not one of IMAGE_HOMES", home[1])
	}
	return home[1], others
}

// foreignHomeRefs lists every reference text makes to another home: its registry path, or the
// raw-file host its git mirror serves install.sh from.
func foreignHomeRefs(text string, others []string) []string {
	var found []string
	for _, h := range others {
		for _, needle := range []string{h + "/", rawBaseOfHome[h]} {
			if strings.Contains(text, needle) {
				found = append(found, needle)
			}
		}
	}
	return found
}

func TestForeignHomeRefsFindsTheOtherHome(t *testing.T) {
	others := []string{"ghcr.io/chetto1983"}
	stale := "AURA_IMAGE=ghcr.io/chetto1983/aura:edge\ncurl -fsSL https://raw.githubusercontent.com/chetto1983/Aura/master/scripts/install.sh"
	if got := foreignHomeRefs(stale, others); len(got) != 2 {
		t.Fatalf("a stale file yielded %v, want its registry path and its raw base", got)
	}
	if got := foreignHomeRefs("AURA_IMAGE=registry.gitlab.com/chetto1983/aura/aura:edge", others); len(got) != 0 {
		t.Fatalf("a file on the home yielded %v", got)
	}
}

// Every default a host receives names the home the updater moves hosts to, so a fresh install
// and an updated host pull the same images (docs/superpowers/specs/2026-10-13-gitlab-image-bridge-design.md).
// The updater declares every home by design, so its IMAGE_HOMES line is not read; the one-time
// bridge patch names both homes by design and is not shipped.
func TestShippedDefaultsNameTheUpdatersImageHome(t *testing.T) {
	root := repoRootForTest(t)
	home, others := imageHomes(t, root)
	shipped := []string{"scripts/install.sh", "scripts/install_env.sh", ".env.example", "packages/create-aura/README.md"}
	for _, pattern := range []string{"compose*.yaml", "deploy/*", "packages/create-aura/src/*.ts"} {
		matches, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil || len(matches) == 0 {
			t.Fatalf("glob %s: %v %v", pattern, matches, err)
		}
		for _, m := range matches {
			rel, _ := filepath.Rel(root, m)
			shipped = append(shipped, filepath.ToSlash(rel))
		}
	}
	declaration := regexp.MustCompile(`(?m)^IMAGE_HOMES=.*$`)
	for _, rel := range shipped {
		if rel == "deploy/aura-registry-bridge.sh" {
			continue
		}
		text := readProjectFile(t, root, rel)
		if rel == "deploy/aura-image-update.sh" {
			text = declaration.ReplaceAllString(text, "")
		}
		if refs := foreignHomeRefs(text, others); len(refs) > 0 {
			t.Errorf("%s still names %v; every shipped default names %s", rel, refs, home)
		}
	}
}
