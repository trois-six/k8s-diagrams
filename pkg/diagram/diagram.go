package diagram

import (
	"fmt"

	"github.com/Trois-Six/k8s-diagrams/pkg/discovery"
	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/k8s"
)

const (
	NSFontSize = 8
)

type Diagram struct {
	filename          string
	outputDir         string
	namespaces        map[string]*diagram.Node
	namespaceGroups   map[string]*diagram.Group
	daemonSets        map[string]*diagram.Node
	daemonSetGroups   map[string]*diagram.Group
	deployments       map[string]*diagram.Node
	replicaSets       map[string]*diagram.Node
	replicaSetGroups  map[string]*diagram.Group
	statefulSets      map[string]*diagram.Node
	statefulSetGroups map[string]*diagram.Group
	diag              *diagram.Diagram
}

func NewDiagram(outputDir, filename, label string) (*Diagram, error) {
	d, err := diagram.New(
		diagram.Filename(filename),
		diagram.Label(label),
		diagram.Direction("TB"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating diagram: %w", err)
	}

	return &Diagram{
		filename:          filename,
		outputDir:         outputDir,
		namespaces:        make(map[string]*diagram.Node),
		namespaceGroups:   make(map[string]*diagram.Group),
		daemonSets:        make(map[string]*diagram.Node),
		daemonSetGroups:   make(map[string]*diagram.Group),
		deployments:       make(map[string]*diagram.Node),
		replicaSets:       make(map[string]*diagram.Node),
		replicaSetGroups:  make(map[string]*diagram.Group),
		statefulSets:      make(map[string]*diagram.Node),
		statefulSetGroups: make(map[string]*diagram.Group),
		diag:              d,
	}, nil
}

func (d *Diagram) GenerateDiagram(namespace string, o *discovery.Objects) {
	for _, ns := range o.Namespaces.Items {
		if ns.Name != namespace {
			continue
		}

		d.namespaces[ns.Name] = k8s.Group.Ns(diagram.NodeLabel(ns.Name))
		d.namespaceGroups[ns.Name] = diagram.NewGroup(ns.Name, func(o *diagram.GroupOptions) {
			o.Font = diagram.Font{
				Name:  "Sans-Serif",
				Size:  NSFontSize,
				Color: "#2D3436",
			}
		}).Label(ns.Name)
		d.namespaceGroups[ns.Name].Add(d.namespaces[ns.Name])
		d.diag.Group(d.namespaceGroups[ns.Name])

		d.GenerateDeployments(namespace, o.Deployments)
		d.GenerateDaemonSets(namespace, o.DaemonSets)
		d.GenerateReplicaSets(namespace, o.ReplicaSets)
		d.GenerateStatefulSets(namespace, o.StatefulSets)
		d.GeneratePods(namespace, o.Pods)
	}
}

func (d *Diagram) RenderDiagram() error {
	if err := d.diag.Render(); err != nil {
		return fmt.Errorf("rendering diagram: %w", err)
	}

	return nil
}
