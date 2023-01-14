package applications

// DataType indicates a PromCommand is for table view or for graph view in data visualization platform like Grafana
// Basically,
//  - TABLE_DATA is for raw table data query
//  - GRAPH_DATA is for time-bounding graph data query
// 	- LABEL_VALUES_DATA is for label values data query like fetching data for label selector options at the top of Grafana dashboard
type DataType int

const (
	TABLE_DATA DataType = iota + 1
	GRAPH_DATA
	LABEL_VALUES_DATA
)
