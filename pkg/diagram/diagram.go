package diagram

import (
	"fmt"

	"github.com/Trois-Six/k8s-diagrams/pkg/discovery"
	"github.com/blushft/go-diagrams/diagram"
)

const (
	nodeFontSize  = 10
	groupFontSize = 10
	nodeWidth     = 1.2
)

type Diagram struct {
	filename          string
	outputDir         string
	namespaceGroups   map[string]*diagram.Group
	daemonSets        map[string]*diagram.Node
	daemonSetGroups   map[string]*diagram.Group
	deployments       map[string]*diagram.Node
	endpoints         map[string]*diagram.Node
	pods              map[string]*diagram.Node
	replicaSets       map[string]*diagram.Node
	replicaSetGroups  map[string]*diagram.Group
	services          map[string]*diagram.Node
	statefulSets      map[string]*diagram.Node
	statefulSetGroups map[string]*diagram.Group
	diag              *diagram.Diagram
}

func NewDiagram(outputDir, filename, label string) (*Diagram, error) {
	d, err := diagram.New(
		diagram.Filename(filename),
		diagram.Label(label),
		diagram.Direction("TB"),
		func(options *diagram.Options) {
			options.Name = outputDir
			options.Attributes["nodesep"] = "1"
		},
	)
	if err != nil {
		return nil, fmt.Errorf("creating diagram: %w", err)
	}

	return &Diagram{
		filename:          filename,
		outputDir:         outputDir,
		namespaceGroups:   make(map[string]*diagram.Group),
		daemonSets:        make(map[string]*diagram.Node),
		daemonSetGroups:   make(map[string]*diagram.Group),
		endpoints:         make(map[string]*diagram.Node),
		deployments:       make(map[string]*diagram.Node),
		pods:              make(map[string]*diagram.Node),
		replicaSets:       make(map[string]*diagram.Node),
		replicaSetGroups:  make(map[string]*diagram.Group),
		services:          make(map[string]*diagram.Node),
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

		d.namespaceGroups[ns.Name] = diagram.NewGroup(ns.Name, func(o *diagram.GroupOptions) {
			o.Font = diagram.Font{
				Size: groupFontSize,
			}
			o.BackgroundColor = "#E0ECF4"
		}).Label(ns.Name)
		d.diag.Group(d.namespaceGroups[ns.Name])

		d.GenerateDeployments(namespace, o.Deployments)
		d.GenerateDaemonSets(namespace, o.DaemonSets)
		d.GenerateReplicaSets(namespace, o.ReplicaSets)
		d.GenerateStatefulSets(namespace, o.StatefulSets)
		d.GeneratePods(namespace, o.Pods)
		d.GenerateServices(namespace, o.Services, o.Endpoints)
	}
}

func (d *Diagram) RenderDiagram() error {
	if err := d.diag.Render(); err != nil {
		return fmt.Errorf("rendering diagram: %w", err)
	}

	return nil
}
