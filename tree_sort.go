package prometheus_backfill

import (
	io_prometheus_client "github.com/prometheus/client_model/go"
)

type node struct {
	value []*io_prometheus_client.Metric
	left  *node
	right *node
}

type bst struct {
	root   *node
	length int64
}

func (t *bst) insert(v []*io_prometheus_client.Metric) {
	if len(v) == 0 {
		ErrLog("Tried insertion of an empty slice")
		return // No insert for empty slices
	}
	if t.root == nil {
		t.root = &node{v, nil, nil}
		t.length = 0
		return
	}
	current := t.root
	t.length++
	for {
		if *v[0].TimestampMs < *current.value[0].TimestampMs {
			if current.left == nil {
				current.left = &node{v, nil, nil}
				return
			}
			current = current.left
		} else {
			if current.right == nil {
				current.right = &node{v, nil, nil}
				return
			}
			current = current.right
		}
	}
}

func (t *bst) inorder(visit func(metric []*io_prometheus_client.Metric)) {
	var traverse func(*node)
	traverse = func(current *node) {
		if current == nil {
			return
		}
		traverse(current.left)
		visit(current.value)
		traverse(current.right)
	}
	traverse(t.root)
}
