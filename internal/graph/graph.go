package graph

import (
    "fmt"
    "sort"

    "github.com/dever-labs/devx/internal/config"
)

type Node struct {
    Name      string
    Kind      string
    DependsOn []string
}

type Graph struct {
    Nodes map[string]Node
}

func Build(profile *config.Profile) (*Graph, error) {
    nodes := make(map[string]Node)

    if profile == nil {
        return &Graph{Nodes: nodes}, nil
    }

    for name, svc := range profile.Services {
        nodes[name] = Node{
            Name:      name,
            Kind:      "service",
            DependsOn: append([]string{}, svc.DependsOn...),
        }
    }

    for name := range profile.Deps {
        if _, ok := nodes[name]; ok {
            return nil, fmt.Errorf("name '%s' is used by both service and dep", name)
        }
        nodes[name] = Node{
            Name:      name,
            Kind:      "dep",
            DependsOn: nil,
        }
    }

    return &Graph{Nodes: nodes}, nil
}

func TopoSort(g *Graph) ([]string, error) {
    if g == nil {
        return nil, nil
    }

    indegree := map[string]int{}
    adj := map[string][]string{}

    for name, node := range g.Nodes {
        indegree[name] = 0
        for _, dep := range node.DependsOn {
            adj[dep] = append(adj[dep], name)
        }
    }

    for name, node := range g.Nodes {
        for _, dep := range node.DependsOn {
            if _, ok := indegree[dep]; !ok {
                return nil, fmt.Errorf("unknown dependency '%s' for '%s'", dep, name)
            }
            indegree[name]++
        }
    }

    var queue []string
    for name, count := range indegree {
        if count == 0 {
            queue = append(queue, name)
        }
    }
    sort.Strings(queue)

    var order []string
    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]
        order = append(order, current)

        for _, next := range adj[current] {
            indegree[next]--
            if indegree[next] == 0 {
                queue = append(queue, next)
            }
        }
        sort.Strings(queue)
    }

    if len(order) != len(g.Nodes) {
        return nil, fmt.Errorf("dependency cycle detected")
    }

    return order, nil
}
