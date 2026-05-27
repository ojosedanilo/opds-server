package main

// Parser YAML sem dependências externas.
// Estratégia: duas passagens simples.
// 1ª: agrupa linhas em blocos de chave→valor ou chave→lista
// Suporta um nível de aninhamento (seção: subchave:)

import (
	"bufio"
	"os"
	"strings"
)

func loadConfig(path string) Config {
	d := defaultConfig()
	m, lists, err := parseYAML(path)
	if err != nil {
		if !os.IsNotExist(err) {
			panic("erro ao ler " + path + ": " + err.Error())
		}
		return d
	}

	str := func(key, def string) string {
		if v, ok := m[key]; ok && v != "" {
			return v
		}
		return def
	}
	boolean := func(key string, def bool) bool {
		if v, ok := m[key]; ok {
			return v == "true" || v == "yes" || v == "1"
		}
		return def
	}
	slice := func(key string, def []string) []string {
		if v, ok := lists[key]; ok && len(v) > 0 {
			return v
		}
		return def
	}

	return Config{
		Title:    str("title", d.Title),
		Port:     str("port", d.Port),
		BooksDir: str("books_dir", d.BooksDir),
		CORS: CORSConfig{
			Enabled:        boolean("cors.enabled", d.CORS.Enabled),
			AllowedOrigins: slice("cors.allowed_origins", d.CORS.AllowedOrigins),
			AllowedMethods: slice("cors.allowed_methods", d.CORS.AllowedMethods),
			AllowedHeaders: slice("cors.allowed_headers", d.CORS.AllowedHeaders),
		},
	}
}

// parseYAML retorna dois maps:
//   scalars: "section.key" → "value"
//   lists:   "section.key" → []string
func parseYAML(path string) (scalars map[string]string, lists map[string][]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scalars = map[string]string{}
	lists = map[string][]string{}

	section := ""   // seção de nível 0 atual (ex: "cors")
	listKey := ""   // chave de lista atual (ex: "cors.allowed_origins")

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		raw := strings.TrimRight(line, " \t\r")
		trimmed := strings.TrimSpace(raw)

		// Ignora vazios e comentários
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		// Item de lista
		if strings.HasPrefix(trimmed, "- ") {
			if listKey != "" {
				item := unquote(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
				lists[listKey] = append(lists[listKey], item)
			}
			continue
		}

		// Linha com "key: valor" ou "key:"
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx <= 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:colonIdx])
		val := strings.TrimSpace(trimmed[colonIdx+1:])
		// Remove comentário inline
		if ci := strings.Index(val, " #"); ci >= 0 {
			val = strings.TrimSpace(val[:ci])
		}

		if indent == 0 {
			// Chave de nível raiz
			section = ""
			listKey = ""
			if val == "" {
				// Início de seção
				section = key
			} else {
				scalars[key] = unquote(val)
			}
		} else {
			// Sub-chave dentro de seção
			fullKey := key
			if section != "" {
				fullKey = section + "." + key
			}
			listKey = ""
			if val == "" {
				// Sub-chave que será seguida por lista
				listKey = fullKey
			} else {
				scalars[fullKey] = unquote(val)
			}
		}
	}
	return scalars, lists, scanner.Err()
}

func unquote(s string) string {
	if len(s) >= 2 {
		q := s[0]
		if (q == '"' || q == '\'') && s[len(s)-1] == q {
			return s[1 : len(s)-1]
		}
	}
	return s
}
