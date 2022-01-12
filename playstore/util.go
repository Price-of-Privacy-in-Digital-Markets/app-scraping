package playstore

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var ErrAppNotFound error = errors.New("app not found")

func pluck(val interface{}, path ...int) (interface{}, error) {
	current := val

	for _, p := range path {
		current_slice, ok := current.([]interface{})
		if !ok {
			return nil, fmt.Errorf("pluck: not a slice")
		}

		if p < len(current_slice) {
			current = current_slice[p]
		} else {
			return nil, fmt.Errorf("pluck: index out of range")
		}
	}

	return current, nil
}

func pluckPanic(val interface{}, path ...int) interface{} {
	ret, err := pluck(val, path...)
	if err != nil {
		panic(err)
	}
	return ret
}

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
