// Package diagram draws Kubernetes objects as a Graphviz diagram.
package diagram

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/blushft/go-diagrams/diagram"
	"github.com/blushft/go-diagrams/nodes/apps"

	"github.com/trois-six/k8s-diagrams/internal/discovery"
)

const (
	nodeFontSize  = 10
	groupFontSize = 10
	edgeFontSize  = 6
	nodeWidth     = 1.2
	nodeHeight    = 1.4
	// labelLineHeight and labelCharWidth are the height of a line and the average width of a character
	// of a node label, in inches.
	labelLineHeight = 0.15
	labelCharWidth  = 0.065
	// labelMargin is the number of characters left as margin around a node label.
	labelMargin = 2
	// labelLineLength is the number of characters of a label line fitting in a node:
	// nodeWidth/labelCharWidth - labelMargin.
	labelLineLength = 16
	namespaceColor  = "#E0ECF4"
	setColor        = "#9EBCDA"
)

// Kinds of the drawn nodes.
const (
	kindConfigMap             = "ConfigMap"
	kindDaemonSet             = "DaemonSet"
	kindDeployment            = "Deployment"
	kindExternalName          = "ExternalName"
	kindGateway               = "Gateway"
	kindGatewayClass          = "GatewayClass"
	kindHostPath              = "HostPath"
	kindIngress               = "Ingress"
	kindIngressClass          = "IngressClass"
	kindNFS                   = "NFS"
	kindPersistentVolume      = "PersistentVolume"
	kindPersistentVolumeClaim = "PersistentVolumeClaim"
	kindPod                   = "Pod"
	kindReplicaSet            = "ReplicaSet"
	kindSecret                = "Secret"
	kindService               = "Service"
	kindStatefulSet           = "StatefulSet"
	kindStorageClass          = "StorageClass"
	kindVolume                = "Volume"
)

// icon creates a node with a given icon, such as k8s.Compute.Pod.
type icon func(...diagram.NodeOption) *diagram.Node

// objectKey identifies a drawn Kubernetes object.
type objectKey struct {
	kind, namespace, name string
}

type edgeKey struct {
	from, to *diagram.Node
}

// edge is an edge between two nodes, whose labels are merged when the same nodes are connected several times.
type edge struct {
	labels  []string
	reverse bool
	dashed  bool
}

// Diagram is a diagram of the objects of a Kubernetes namespace.
type Diagram struct {
	diag      *diagram.Diagram
	namespace string
	nsGroup   *diagram.Group
	internet  *diagram.Node
	nodes     map[objectKey]*diagram.Node
	groups    map[objectKey]*diagram.Group
	podGroups map[string]*diagram.Group // The group in which each pod is drawn.
	edges     map[edgeKey]*edge
	edgeOrder []edgeKey
}

// New creates a diagram rendered as filename.dot in outputDir.
func New(outputDir, filename, label string) (*Diagram, error) {
	d, err := diagram.New(
		diagram.Filename(filename),
		diagram.Label(quote(label)),
		diagram.Direction("TB"),
		func(o *diagram.Options) {
			o.Name = outputDir
			o.Attributes["nodesep"] = "1"
			o.Attributes["splines"] = "curved"
		},
	)
	if err != nil {
		return nil, fmt.Errorf("creating diagram: %w", err)
	}

	return &Diagram{
		diag:      d,
		nodes:     make(map[objectKey]*diagram.Node),
		groups:    make(map[objectKey]*diagram.Group),
		podGroups: make(map[string]*diagram.Group),
		edges:     make(map[edgeKey]*edge),
	}, nil
}

// Build draws the objects: the workloads and their pods, then the network or the storage objects,
// depending on the mode the objects were discovered for.
func (d *Diagram) Build(o *discovery.Objects) {
	d.namespace = o.Namespace
	d.nsGroup = newGroup("Namespace-"+o.Namespace, o.Namespace, namespaceColor)
	d.diag.Group(d.nsGroup)

	d.addDeployments(o.Deployments)
	d.addDaemonSets(o.DaemonSets)
	d.addReplicaSets(o.ReplicaSets)
	d.addStatefulSets(o.StatefulSets)
	d.addPods(o.Pods)

	switch o.Mode {
	case discovery.ModeNetwork:
		d.addServices(o.Services)
		d.addEndpointSlices(o.EndpointSlices)
		d.addIngresses(o.Ingresses, o.IngressClasses)
		d.addGateways(o.Gateways, o.GatewayClasses)
		d.addRoutes(o.Routes)
	case discovery.ModeStorage:
		d.addPersistentVolumeClaims(o.PersistentVolumeClaims, o.PersistentVolumes, o.StorageClasses)
		d.addPodVolumes(o.Pods)
	}

	d.flushEdges()
}

// Render writes the diagram to the output directory.
func (d *Diagram) Render() error {
	if err := d.diag.Render(); err != nil {
		return fmt.Errorf("rendering diagram: %w", err)
	}

	return nil
}

func newGroup(id, label, color string) *diagram.Group {
	return diagram.NewGroup(id, func(o *diagram.GroupOptions) {
		o.Font = diagram.Font{Size: groupFontSize}
		o.BackgroundColor = color
	}).Label(quote(label))
}

// addNode creates a node for a Kubernetes object of the namespace and adds it to the namespace group.
func (d *Diagram) addNode(kind, name string, icon icon) *diagram.Node {
	n := newNode(icon, name)
	d.nodes[objectKey{kind: kind, namespace: d.namespace, name: name}] = n
	d.nsGroup.Add(n)

	return n
}

// node returns the node of a Kubernetes object.
// Objects outside of the drawn namespace are drawn outside of the namespace group, on first use.
func (d *Diagram) node(kind, namespace, name string, icon icon) (*diagram.Node, bool) {
	key := objectKey{kind: kind, namespace: namespace, name: name}
	if n, ok := d.nodes[key]; ok {
		return n, true
	}

	if namespace == d.namespace {
		return nil, false
	}

	label := name
	if namespace != "" {
		label = namespace + "/\\n" + name
	}

	n := newNode(icon, label)
	d.nodes[key] = n
	d.diag.Add(n)

	return n, true
}

func newNode(icon icon, label string) *diagram.Node {
	n := icon(
		diagram.SetFontOptions(diagram.Font{Size: nodeFontSize}),
	)
	setLabel(n, label)

	return n
}

// quote quotes a label, as gographviz does not quote every label that needs to be, such as "web.v2".
// strconv.Quote cannot be used, as it would escape the line breaks ("\\n").
func quote(label string) string {
	return `"` + label + `"`
}

// setLabel sets the label of a node, wrapping its long lines, and sizes the node for the label
// not to overflow on the other nodes: the node is higher for each line of the label,
// and wider for the lines that could not be wrapped, which makes its icon bigger.
func setLabel(n *diagram.Node, label string) {
	var lines []string
	for line := range strings.SplitSeq(label, `\n`) {
		lines = append(lines, wrapLine(line, labelLineLength)...)
	}

	longest := len(slices.MaxFunc(lines, func(a, b string) int { return cmp.Compare(len(a), len(b)) }))

	n.Options.Label = quote(strings.Join(lines, `\n`))
	n.Options.Height = nodeHeight + labelLineHeight*float64(len(lines)-1)
	n.Options.Width = max(nodeWidth, labelCharWidth*float64(longest+labelMargin))
}

// wrapLine wraps a line after its separators, such as dashes and dots, in lines of at most length characters
// when possible.
func wrapLine(line string, length int) []string {
	var lines []string

	start, lastBreak := 0, -1

	for i, r := range line {
		if i-start >= length && lastBreak > start {
			lines = append(lines, line[start:lastBreak])
			start = lastBreak
		}

		if strings.ContainsRune("-./: ", r) {
			lastBreak = i + 1
		}
	}

	return append(lines, line[start:])
}

// labelOf returns the label of a node.
func labelOf(n *diagram.Node) string {
	return strings.Trim(n.Options.Label, `"`)
}

func (d *Diagram) internetNode() *diagram.Node {
	if d.internet == nil {
		d.internet = apps.Network.Internet(diagram.NodeLabel(quote("Internet")))
		d.diag.Add(d.internet)
	}

	return d.internet
}

// connectFromInternet draws the traffic from the Internet to a node exposed with the given IP or hostname.
func (d *Diagram) connectFromInternet(to *diagram.Node, address string) {
	if address == "" {
		return
	}

	d.connectTraffic(d.internetNode(), to, address)
}

// connectTraffic draws the traffic going from source to target.
// The diagram is laid out top-down from the workloads to their pods, so the traffic is laid out
// bottom-up: from the Internet at the bottom, through the gateways, ingresses and services, to the pods.
func (d *Diagram) connectTraffic(source, target *diagram.Node, label string, opts ...func(*edge)) {
	d.connect(target, source, label, append(opts, reversed)...)
}

// connect draws an edge from a node to a node laid out below it, merging the labels of identical edges.
func (d *Diagram) connect(from, to *diagram.Node, label string, opts ...func(*edge)) {
	key := edgeKey{from: from, to: to}

	e, ok := d.edges[key]
	if !ok {
		e = &edge{}
		d.edges[key] = e
		d.edgeOrder = append(d.edgeOrder, key)
	}

	if label != "" && !slices.Contains(e.labels, label) {
		e.labels = append(e.labels, label)
	}

	for _, opt := range opts {
		opt(e)
	}
}

// reversed draws the arrow from the end to the start of the edge, without changing the layout.
func reversed(e *edge) { e.reverse = true }

// dashed draws a dashed edge.
func dashed(e *edge) { e.dashed = true }

func (d *Diagram) flushEdges() {
	for _, key := range d.edgeOrder {
		e := d.edges[key]

		d.diag.ConnectByID(key.from.ID(), key.to.ID(), func(o *diagram.EdgeOptions) {
			o.Font.Size = edgeFontSize

			if len(e.labels) > 0 {
				o.Attributes["xlabel"] = quote(strings.Join(e.labels, `\n`))
			}

			if e.reverse {
				o.Forward, o.Reverse = false, true
			}

			if e.dashed {
				o.Style = "dashed"
			}
		})
	}
}
