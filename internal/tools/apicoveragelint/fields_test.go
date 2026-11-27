package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// The published layout: openapi.yaml only $refs path items of per-area files
// next to it, which carry their own components.
const mainSpec = `openapi: 3.0.0
paths:
  /tags:
    $ref: tags.yaml#/paths/~1tags
  /tags/{id}:
    $ref: tags.yaml#/paths/~1tags~1{id}
  /upload:
    post:
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                Name:
                  type: string
                file:
                  type: string
                  format: binary
components: {}
`

const tagsSpec = `openapi: 3.0.0
paths:
  /tags:
    get:
      responses: {}
    post:
      requestBody:
        $ref: '#/components/requestBodies/tagCreate'
  /tags/{id}:
    put:
      requestBody:
        content:
          application/json:
            schema:
              allOf:
                - $ref: '#/components/schemas/base'
                - type: object
                  properties:
                    Color:
                      type: string
    delete:
      requestBody:
        content:
          text/plain:
            schema:
              type: string
components:
  requestBodies:
    tagCreate:
      content:
        application/json:
          schema:
            $ref: '#/components/schemas/base'
  schemas:
    base:
      type: object
      properties:
        name:
          type: string
        id:
          type: integer
`

func TestReadSpecOperations(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"openapi.yaml": mainSpec, "tags.yaml": tagsSpec})

	ops, err := readSpecOperations(filepath.Join(dir, "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	want := []operation{
		{Method: "POST", Path: "/tags", Fields: []string{"id", "name"}},
		{Method: "PUT", Path: "/tags/{id}", Fields: []string{"Color", "id", "name"}},
		{Method: "POST", Path: "/upload", Fields: []string{"Name", "file"}},
	}
	if !reflect.DeepEqual(ops, want) {
		t.Errorf("operations:\n got %+v\nwant %+v", ops, want)
	}
}

func TestReadSpecOperationsMissingRefFile(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"openapi.yaml": mainSpec})
	if _, err := readSpecOperations(filepath.Join(dir, "openapi.yaml")); err == nil {
		t.Fatal("want an error for the missing tags.yaml")
	}
}

func TestReadSpecOperationsRemoteRef(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"openapi.yaml": `paths:
  /tags:
    $ref: https://example.com/tags.yaml#/paths/~1tags
`})
	_, err := readSpecOperations(filepath.Join(dir, "openapi.yaml"))
	if err == nil || !strings.Contains(err.Error(), "remote $ref") {
		t.Fatalf("want a remote $ref error, got %v", err)
	}
}

// providerSource uses every way the provider names a request field.
const providerSource = `package internal

type tagPayload struct {
	Name  string ` + "`json:\"name,omitempty\"`" + `
	Skip  string ` + "`json:\"-\"`" + `
	Plain string
}

var attrToAPI = map[string]string{"color_attr": "ColorFromMapValue"}

func build(w writer) {
	body := map[string]interface{}{"FromMapKey": 1}
	body["FromIndex"] = 2
	_ = w.WriteField("FromWriteField", "x")
	_, _ = w.CreateFormFile("FromFormFile", "f")
	addField := func(name, value string) {}
	addField("FromAddField", "v")
	_ = [2]string{"FromPairLit", "v"}
	_ = multipartFile{Field: "FromMultipartFile"}
	_ = "/tags/{id}"
}
`

func TestProviderFieldNames(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"provider.go":      providerSource,
		"provider_test.go": "package internal\n\nvar _ = map[string]int{\"OnlyInTest\": 1}\n",
	})
	names, err := providerFieldNames(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"name", "colorfrommapvalue", "frommapkey", "fromindex", "fromwritefield",
		"fromformfile", "fromaddfield", "frompairlit", "frommultipartfile",
	}
	for _, n := range want {
		if !names[n] {
			t.Errorf("%q not collected", n)
		}
	}
	for _, n := range []string{"-", "skip", "plain", "onlyintest", "/tags/{id}"} {
		if names[n] {
			t.Errorf("%q collected, want it left out", n)
		}
	}
}

func TestReadFieldIgnore(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ignore-fields.txt")
	writeFiles(t, dir, map[string]string{"ignore-fields.txt": "# comment\n\nput /tags/{id} Color  # reason\n* * id # read-only\n"})
	got, err := readFieldIgnore(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].String() != "PUT /tags/{id} Color" || got[1].String() != "* * id" {
		t.Errorf("entries: %v", got)
	}

	for _, bad := range []string{"PUT /tags Color\n", "PUT /tags # no field\n"} {
		writeFiles(t, dir, map[string]string{"ignore-fields.txt": bad})
		if _, err := readFieldIgnore(file); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

func TestCheckFields(t *testing.T) {
	ops := []operation{
		{Method: "POST", Path: "/tags", Fields: []string{"id", "name"}},
		{Method: "PUT", Path: "/tags/{id}", Fields: []string{"Color", "id", "name"}},
		// The same operation from a second spec counts once.
		{Method: "PUT", Path: "/tags/{id}", Fields: []string{"Color", "id", "name", "Extra"}},
		// Paths the provider does not call are the path check's business.
		{Method: "POST", Path: "/uncovered", Fields: []string{"x"}},
	}
	patterns := [][]string{{"tags"}, {"tags", param}}
	sent := map[string]bool{"name": true}
	ignores := []*fieldIgnore{
		{Method: "*", Path: "*", Field: "ID"},
		{Method: "PUT", Path: "/tags/{id}", Field: "Gone"},
	}

	r := checkFields(ops, patterns, sent, ignores)
	if r.Operations != 2 || r.Fields != 6 || r.Sent != 2 || r.Ignored != 2 {
		t.Errorf("counts: %+v", r)
	}
	if want := []string{"PUT /tags/{id} Color", "PUT /tags/{id} Extra"}; !reflect.DeepEqual(r.Missing, want) {
		t.Errorf("missing: got %v, want %v", r.Missing, want)
	}
	if want := []string{"PUT /tags/{id} Gone"}; !reflect.DeepEqual(r.Stale, want) {
		t.Errorf("stale: got %v, want %v", r.Stale, want)
	}
}
