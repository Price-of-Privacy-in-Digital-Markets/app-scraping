package playstore

import (
	"errors"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var ErrAppNotFound error = errors.New("app not found")

func textFromHTML(description string) (string, error) {
	fragment, err := html.ParseFragment(strings.NewReader(description), nil)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	var visit_node func(*html.Node)
	visit_node = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}

		if n.Type == html.ElementNode && n.DataAtom == atom.Br {
			sb.WriteString("\n")
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit_node(c)
		}

		if n.Type == html.ElementNode && n.DataAtom == atom.P {
			sb.WriteString("\n")
		}
	}

	visitor := func(nodes []*html.Node) {
		for _, n := range nodes {
			visit_node(n)
		}
	}

	visitor(fragment)
	return sb.String(), nil
}
