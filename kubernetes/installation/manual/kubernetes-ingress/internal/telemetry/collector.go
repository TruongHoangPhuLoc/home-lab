// Package telemetry provides functionality for collecting and exporting NIC telemetry data.
package telemetry

import (
	"context"
	"io"
	"runtime"
	"time"

	tel "github.com/nginxinc/telemetry-exporter/pkg/telemetry"

	"github.com/nginxinc/kubernetes-ingress/internal/configs"

	k8s_nginx "github.com/nginxinc/kubernetes-ingress/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/golang/glog"
)

// Option is a functional option used for configuring TraceReporter.
type Option func(*Collector) error

// WithExporter configures telemetry collector to use given exporter.
//
// This may change in the future when we use exporter implemented
// in the external module.
func WithExporter(e Exporter) Option {
	return func(c *Collector) error {
		c.Exporter = e
		return nil
	}
}

// Collector is NIC telemetry data collector.
type Collector struct {
	// Exporter is a temp exporter for exporting telemetry data.
	// The concrete implementation will be implemented in a separate module.
	Exporter Exporter

	// Configuration for the collector.
	Config CollectorConfig
}

// CollectorConfig contains configuration options for a Collector
type CollectorConfig struct {
	// K8sClientReader is a kubernetes client.
	K8sClientReader kubernetes.Interface

	// CustomK8sClientReader is a kubernetes client for our CRDs.
	// Note: May not need this client.
	CustomK8sClientReader k8s_nginx.Interface

	// Period to collect telemetry
	Period time.Duration

	Configurator *configs.Configurator

	// Version represents NIC version.
	Version string

	// PodNSName represents NIC Pod's NamespacedName.
	PodNSName types.NamespacedName
}

// NewCollector takes 0 or more options and creates a new TraceReporter.
// If no options are provided, NewReporter returns TraceReporter
// configured to gather data every 24h.
func NewCollector(cfg CollectorConfig, opts ...Option) (*Collector, error) {
	c := Collector{
		Exporter: &StdoutExporter{Endpoint: io.Discard},
		Config:   cfg,
	}
	for _, o := range opts {
		if err := o(&c); err != nil {
			return nil, err
		}
	}
	return &c, nil
}

// Start starts running NIC Telemetry Collector.
func (c *Collector) Start(ctx context.Context) {
	wait.JitterUntilWithContext(ctx, c.Collect, c.Config.Period, 0.1, true)
}

// Collect collects and exports telemetry data.
// It exports data using provided exporter.
func (c *Collector) Collect(ctx context.Context) {
	glog.V(3).Info("Collecting telemetry data")
	report, err := c.BuildReport(ctx)
	if err != nil {
		glog.Errorf("Error collecting telemetry data: %v", err)
	}

	nicData := Data{
		tel.Data{
			ProjectName:         report.Name,
			ProjectVersion:      c.Config.Version,
			ProjectArchitecture: runtime.GOARCH,
			ClusterID:           report.ClusterID,
			ClusterVersion:      report.ClusterVersion,
			ClusterPlatform:     report.ClusterPlatform,
			InstallationID:      report.InstallationID,
			ClusterNodeCount:    int64(report.ClusterNodeCount),
		},
		NICResourceCounts{
			VirtualServers:      int64(report.VirtualServers),
			VirtualServerRoutes: int64(report.VirtualServerRoutes),
			TransportServers:    int64(report.TransportServers),
			Replicas:            int64(report.NICReplicaCount),
		},
	}

	err = c.Exporter.Export(ctx, &nicData)
	if err != nil {
		glog.Errorf("Error exporting telemetry data: %v", err)
	}
	glog.V(3).Infof("Telemetry data collected: %+v", nicData)
}

// Report holds collected NIC telemetry data. It is the package internal
// data structure used for decoupling types between the NIC `telemetry`
// package and the imported `telemetry` exporter.
type Report struct {
	Name                string
	Version             string
	Architecture        string
	ClusterID           string
	ClusterVersion      string
	ClusterPlatform     string
	ClusterNodeCount    int
	InstallationID      string
	NICReplicaCount     int
	VirtualServers      int
	VirtualServerRoutes int
	TransportServers    int
}

// BuildReport takes context, collects telemetry data and builds the report.
func (c *Collector) BuildReport(ctx context.Context) (Report, error) {
	vsCount := 0
	vsrCount := 0
	tsCount := 0

	if c.Config.Configurator != nil {
		vsCount, vsrCount = c.Config.Configurator.GetVirtualServerCounts()
		tsCount = c.Config.Configurator.GetTransportServerCounts()
	}

	clusterID, err := c.ClusterID(ctx)
	if err != nil {
		glog.Errorf("Error collecting telemetry data: ClusterID: %v", err)
	}

	nodes, err := c.NodeCount(ctx)
	if err != nil {
		glog.Errorf("Error collecting telemetry data: Nodes: %v", err)
	}

	version, err := c.ClusterVersion()
	if err != nil {
		glog.Errorf("Error collecting telemetry data: K8s Version: %v", err)
	}

	platform, err := c.Platform(ctx)
	if err != nil {
		glog.Errorf("Error collecting telemetry data: Platform: %v", err)
	}

	replicas, err := c.ReplicaCount(ctx)
	if err != nil {
		glog.Errorf("Error collecting telemetry data: Replicas: %v", err)
	}

	installationID, err := c.InstallationID(ctx)
	if err != nil {
		glog.Errorf("Error collecting telemetry data: InstallationID: %v", err)
	}

	return Report{
		Name:                "NIC",
		Version:             c.Config.Version,
		Architecture:        runtime.GOARCH,
		ClusterID:           clusterID,
		ClusterVersion:      version,
		ClusterPlatform:     platform,
		ClusterNodeCount:    nodes,
		InstallationID:      installationID,
		NICReplicaCount:     replicas,
		VirtualServers:      vsCount,
		VirtualServerRoutes: vsrCount,
		TransportServers:    tsCount,
	}, err
}
