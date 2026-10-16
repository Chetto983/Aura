package filecard

import (
	"bytes"
	"io"
	"os"
	"slices"
)

// sourceExtensionsByLanguage are the source-file extensions Aura reads as text: the ingest
// reads them directly as passages, the upload gate accepts them, and their card names them a
// source file (prd.md §11, 2026-10-16). This is the only copy: services/ingest asks
// aura-filecard -source-extensions for it.
//
// The languages are the common ones of the GitHub-linguist list the operator chose, gist
// ppisarczyk/43962d06686722d26d176fad46879d41 ("Programming Languages and their File
// Extensions"), with that list's extensions for each. Left out: compound suffixes such as
// .cmake.in, which a path's suffix never shows, and .ipynb, a JSON notebook whose outputs are
// base64. Added by hand: the languages that list predates (Fortran, Lisp, Nim, Solidity,
// Svelte, Vim script, Zig); IEC 61131-3 Structured Text, as the ControlForge extension names
// it; the Siemens TIA Portal sources, which only secondary sources document; and CSS, Cypher,
// .jsx and .mjs, which indexing Aura's own repository found missing.
//
// An extension two languages share appears under both. A name says nothing about the bytes:
// a .db is a Siemens data block source and far more often SQLite, which is why describeSource
// and the ingest read these files with a rule that keeps a binary out.
var sourceExtensionsByLanguage = map[string][]string{
	"ABAP":       {".abap"},
	"Ada":        {".ada", ".adb", ".ads"},
	"Apex":       {".cls"},
	"Assembly":   {".a51", ".asm", ".inc", ".nasm"},
	"AutoHotkey": {".ahk", ".ahkl"},
	"Batchfile":  {".bat", ".cmd"},
	"C":          {".c", ".cats", ".h", ".idc", ".w"},
	"C#":         {".cake", ".cs", ".cshtml", ".csx"},
	"C++": {
		".c++", ".cc", ".cp", ".cpp", ".cxx", ".h", ".h++", ".hh", ".hpp", ".hxx", ".inc", ".inl",
		".ipp", ".tcc", ".tpp",
	},
	"Clojure":      {".boot", ".cl2", ".clj", ".cljc", ".cljs", ".cljscm", ".cljx", ".hic"},
	"CMake":        {".cmake"},
	"COBOL":        {".cbl", ".ccp", ".cob", ".cobol", ".cpy"},
	"CoffeeScript": {".cake", ".cjsx", ".coffee", ".cson", ".iced"},
	"Crystal":      {".cr"},
	"CSS":          {".css"},
	"Cuda":         {".cu", ".cuh"},
	"Cypher":       {".cypher"},
	"D":            {".d", ".di"},
	"Dart":         {".dart"},
	"Diff":         {".diff", ".patch"},
	"Dockerfile":   {".dockerfile"},
	"Elixir":       {".ex", ".exs"},
	"Elm":          {".elm"},
	"Erlang":       {".erl", ".es", ".escript", ".hrl", ".xrl", ".yrl"},
	"F#":           {".fs", ".fsi", ".fsx"},
	"Fortran":      {".f", ".f90", ".f95", ".f03", ".for"},
	"GLSL": {
		".fp", ".frag", ".frg", ".fs", ".fsh", ".fshader", ".geo", ".geom", ".glsl", ".glslv",
		".gshader", ".shader", ".vert", ".vrx", ".vsh", ".vshader",
	},
	"Go":      {".go"},
	"Gradle":  {".gradle"},
	"GraphQL": {".graphql"},
	"Groovy":  {".groovy", ".grt", ".gtpl", ".gvy"},
	"Hack":    {".hh", ".php"},
	"Haskell": {".hs", ".hsc"},
	"HCL":     {".hcl", ".tf"},
	"INI":     {".cfg", ".ini", ".prefs", ".pro", ".properties"},
	"Java":    {".java"},
	"JavaScript": {
		".bones", ".es", ".es6", ".frag", ".gs", ".jake", ".js", ".jsb", ".jscad", ".jsfl", ".jsm",
		".jss", ".jsx", ".mjs", ".njs", ".pac", ".sjs", ".ssjs", ".sublime-build",
		".sublime-commands", ".sublime-completions", ".sublime-keymap", ".sublime-macro",
		".sublime-menu", ".sublime-mousemap", ".sublime-project", ".sublime-settings",
		".sublime-theme", ".sublime-workspace", ".sublime_metrics", ".sublime_session", ".xsjs",
		".xsjslib",
	},
	"Julia":           {".jl"},
	"Kotlin":          {".kt", ".ktm", ".kts"},
	"Lisp":            {".lisp", ".lsp"},
	"Lua":             {".fcgi", ".lua", ".nse", ".pd_lua", ".rbxs", ".wlua"},
	"Makefile":        {".d", ".mak", ".mk", ".mkfile"},
	"Matlab":          {".m", ".matlab"},
	"Nim":             {".nim"},
	"Nix":             {".nix"},
	"Objective-C":     {".h", ".m"},
	"Objective-C++":   {".mm"},
	"OCaml":           {".eliom", ".eliomi", ".ml", ".ml4", ".mli", ".mll", ".mly"},
	"Pascal":          {".dfm", ".dpr", ".inc", ".lpr", ".pas", ".pp"},
	"Perl":            {".al", ".cgi", ".fcgi", ".perl", ".ph", ".pl", ".plx", ".pm", ".pod", ".psgi", ".t"},
	"PHP":             {".aw", ".ctp", ".fcgi", ".inc", ".php", ".php3", ".php4", ".php5", ".phps", ".phpt"},
	"PLpgSQL":         {".sql"},
	"PLSQL":           {".pck", ".pkb", ".pks", ".plb", ".pls", ".plsql", ".sql"},
	"PowerShell":      {".ps1", ".psd1", ".psm1"},
	"Prolog":          {".pl", ".pro", ".prolog", ".yap"},
	"Protocol Buffer": {".proto"},
	"Python": {
		".bzl", ".cgi", ".fcgi", ".gyp", ".lmi", ".py", ".pyde", ".pyp", ".pyt", ".pyw", ".rpy",
		".tac", ".wsgi", ".xpy",
	},
	"R":      {".r", ".rd", ".rsx"},
	"Racket": {".rkt", ".rktd", ".rktl", ".scrbl"},
	"Ruby": {
		".builder", ".fcgi", ".gemspec", ".god", ".irbrc", ".jbuilder", ".mspec", ".pluginspec",
		".podspec", ".rabl", ".rake", ".rb", ".rbuild", ".rbw", ".rbx", ".ru", ".ruby", ".thor",
		".watchr",
	},
	"Rust":   {".rs"},
	"Scala":  {".sbt", ".sc", ".scala"},
	"Scheme": {".scm", ".sld", ".sls", ".sps", ".ss"},
	"Shell": {
		".bash", ".bats", ".cgi", ".command", ".fcgi", ".ksh", ".sh", ".tmux", ".tool", ".zsh",
	},
	"Siemens TIA Portal sources":    {".scl", ".awl", ".db", ".udt"},
	"Smalltalk":                     {".cs", ".st"},
	"Solidity":                      {".sol"},
	"SQL":                           {".cql", ".ddl", ".inc", ".prc", ".sql", ".tab", ".udf", ".viw"},
	"Structured Text (IEC 61131-3)": {".st", ".iecst"},
	"Svelte":                        {".svelte"},
	"Swift":                         {".swift"},
	"SystemVerilog":                 {".sv", ".svh", ".vh"},
	"Tcl":                           {".adp", ".tcl", ".tm"},
	"TOML":                          {".toml"},
	"TypeScript":                    {".ts", ".tsx"},
	"Verilog":                       {".v", ".veo"},
	"VHDL":                          {".vhd", ".vhdl", ".vhf", ".vhi", ".vho", ".vhs", ".vht", ".vhw"},
	"Vim script":                    {".vim"},
	"Visual Basic":                  {".bas", ".cls", ".frm", ".frx", ".vb", ".vba", ".vbhtml", ".vbs"},
	"Vue":                           {".vue"},
	"XSLT":                          {".xsl", ".xslt"},
	"YAML":                          {".reek", ".rviz", ".sublime-syntax", ".syntax", ".yaml", ".yaml-tmlanguage", ".yml"},
	"Zig":                           {".zig"},
}

var sourceExtensions = func() map[string]struct{} {
	set := map[string]struct{}{}
	for _, exts := range sourceExtensionsByLanguage {
		for _, ext := range exts {
			set[ext] = struct{}{}
		}
	}
	return set
}()

// SourceExtensions lists every source-file extension, sorted.
func SourceExtensions() []string {
	out := make([]string, 0, len(sourceExtensions))
	for ext := range sourceExtensions {
		out = append(out, ext)
	}
	slices.Sort(out)
	return out
}

func isSourceExtension(ext string) bool {
	_, ok := sourceExtensions[ext]
	return ok
}

// describeSource names a source file and claims nothing about its contents: the ingest reads
// it as passages, and an outline of it has not been measured. A NUL byte outside a UTF-16 BOM
// marks a binary under a source name, as it does in services/ingest/extract.py, and that file
// gets the unread card any unknown file gets. extract.py also reads no text from bytes that
// neither UTF-8 nor cp1252 decodes; such a file still gets a source card here.
func describeSource(req Request, card Card) (Card, error) {
	binary, err := holdsBinary(req)
	if err != nil {
		return card, err
	}
	if binary {
		card.Kind = KindFile
		card.Caveats = append(card.Caveats, unreadCaveat)
	}
	return card, nil
}

func holdsBinary(req Request) (bool, error) {
	file, err := os.Open(req.Path)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()
	buf := make([]byte, 64<<10)
	for first := true; ; first = false {
		n, err := file.Read(buf)
		chunk := buf[:n]
		if first && (bytes.HasPrefix(chunk, []byte{0xFF, 0xFE}) || bytes.HasPrefix(chunk, []byte{0xFE, 0xFF})) {
			return false, nil
		}
		if bytes.IndexByte(chunk, 0) >= 0 {
			return true, nil
		}
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
	}
}
