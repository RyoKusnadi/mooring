package lsp

// Install says how one-click install gets a server into <data>/lsp.
type Install struct {
	Kind string   // "go", "npm" or "rustup"
	Args []string // go: the package; npm: the packages; rustup: the component
}

// Def is one server of the catalog.
type Def struct {
	ID         string
	Name       string
	Endings    []string
	LanguageID string
	Command    []string // program name or path, then arguments
	Install    *Install
	Hint       string // what to do when there is no one-click install
	// InitOptions is merged into initializationOptions of the first initialize.
	InitOptions map[string]any
	Custom      bool
}

func npm(pkgs ...string) *Install { return &Install{Kind: "npm", Args: pkgs} }

// Catalog is the starter set.
func Catalog() []Def {
	vs := []string{"vscode-langservers-extracted"}
	return []Def{
		{ID: "go", Name: "Go (gopls)", Endings: []string{".go"}, LanguageID: "go",
			Command: []string{"gopls", "serve"}, Install: &Install{Kind: "go", Args: []string{"golang.org/x/tools/gopls@latest"}}},
		{ID: "typescript", Name: "TypeScript and JavaScript", LanguageID: "typescript",
			Endings: []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"},
			Command: []string{"typescript-language-server", "--stdio"},
			Install: npm("typescript-language-server@6.1.0", "typescript@5")},
		{ID: "python", Name: "Python (pyright)", Endings: []string{".py"}, LanguageID: "python",
			Command: []string{"pyright-langserver", "--stdio"}, Install: npm("pyright")},
		{ID: "rust", Name: "Rust (rust-analyzer)", Endings: []string{".rs"}, LanguageID: "rust",
			Command: []string{"rust-analyzer"}, Install: &Install{Kind: "rustup", Args: []string{"rust-analyzer"}}},
		{ID: "c", Name: "C and C++ (clangd)", Endings: []string{".c", ".h", ".cc", ".cpp", ".hpp", ".cxx"}, LanguageID: "cpp",
			Command: []string{"clangd"}, Hint: "Install clangd with your package manager (for example brew install llvm), then reopen the file."},
		{ID: "java", Name: "Java (jdtls)", Endings: []string{".java"}, LanguageID: "java",
			Command: []string{"jdtls"}, Hint: "Install Eclipse JDT Language Server (jdtls) with your package manager, then reopen the file."},
		{ID: "lua", Name: "Lua", Endings: []string{".lua"}, LanguageID: "lua",
			Command: []string{"lua-language-server"}, Hint: "Install lua-language-server with your package manager, then reopen the file."},
		{ID: "bash", Name: "Shell (bash-language-server)", Endings: []string{".sh", ".bash"}, LanguageID: "shellscript",
			Command: []string{"bash-language-server", "start"}, Install: npm("bash-language-server")},
		{ID: "html", Name: "HTML", Endings: []string{".html", ".htm"}, LanguageID: "html",
			Command: []string{"vscode-html-language-server", "--stdio"}, Install: npm(vs...)},
		{ID: "css", Name: "CSS, SCSS and Less", Endings: []string{".css", ".scss", ".less"}, LanguageID: "css",
			Command: []string{"vscode-css-language-server", "--stdio"}, Install: npm(vs...)},
		{ID: "json", Name: "JSON", Endings: []string{".json", ".jsonc"}, LanguageID: "json",
			Command: []string{"vscode-json-language-server", "--stdio"}, Install: npm(vs...)},
	}
}
