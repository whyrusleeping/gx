package main

import (
	"fmt"
	"strconv"
	"strings"
)

func parseQueryString(qstr string) ([]string, error) {
	qstr = strings.TrimLeft(qstr, ".")
	if qstr == "" {
		return nil, nil
	}
	var out []string
	for {
		i := strings.IndexAny(qstr, ".[")
		if i == -1 {
			return append(out, qstr), nil
		}

		out = append(out, qstr[:i])
		switch qstr[i] {
		case '[':
			clb := findClosingBracket(qstr[i+1:])
			if clb == -1 {
				return nil, fmt.Errorf("closing bracket not found")
			}
			out = append(out, qstr[i:clb+i+2])
			qstr = qstr[i+clb+3:]
		case '.':
			qstr = qstr[i+1:]
		}
	}
}

func findClosingBracket(a string) int {
	var offset int
	for {
		i := strings.IndexAny(a[offset:], "[]")
		if i == -1 {
			return -1
		}

		if a[offset+i] == ']' {
			return offset + i
		}

		n_i := findClosingBracket(a[offset+i+1:])
		if n_i == -1 {
			return -1
		}

		offset += n_i + i + 2
	}
}

func processJsonQuery(cur interface{}, querystr string) (interface{}, error) {
	query, err := parseQueryString(querystr)
	if err != nil {
		return nil, err
	}

	for i, q := range query {
		switch obj := cur.(type) {
		case map[string]interface{}:
			v, ok := obj[q]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", strings.Join(query[:i+1], "."))
			}

			cur = v
		case []interface{}:
			if q[0] != '[' || q[len(q)-1] != ']' {
				return nil, fmt.Errorf("must use [N] notation for accessing arrays")
			}

			nums := strings.Trim(q, "[]")
			if nums == "" {
				return nil, fmt.Errorf("don't currently support queries on multiple array members")
			}

			var i int
			if strings.Contains(nums, "=") {
				parts := strings.Split(nums, "=")
				if len(parts) != 2 {
					return nil, fmt.Errorf("array queries must contain a single equality operator")
				}
				n, err := matchChild(obj, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
				if err != nil {
					return nil, err
				}

				i = n
			} else {
				n, err := strconv.Atoi(nums)
				if err != nil {
					return nil, err
				}

				i = n
			}

			cur = obj[i]
		default:
			return nil, fmt.Errorf("end of the line?")
		}
	}

	return cur, nil
}

func matchChild(arr []interface{}, query string, val string) (int, error) {
	for i, v := range arr {
		out, err := processJsonQuery(v, query)
		if err == nil && out == val {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no child matching query found")
}
