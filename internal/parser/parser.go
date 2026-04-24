package parser

// Parse detects the language for filePath, parses src, and returns the
// top-level Node tree together with the canonical language name.
func Parse(src []byte, filePath string) ([]*Node, string, error) {
	h, err := For(filePath)
	if err != nil {
		return nil, "", err
	}
	nodes, err := h.Nodes(src, filePath)
	if err != nil {
		return nil, "", err
	}
	return nodes, h.Language(), nil
}

// ListSymbols returns the top-level symbols defined in src for the language
// inferred from filePath.
func ListSymbols(src []byte, filePath string) ([]string, error) {
	h, err := For(filePath)
	if err != nil {
		return nil, err
	}
	return h.Symbols(src)
}
