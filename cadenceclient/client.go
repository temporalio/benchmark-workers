// Package cadenceclient builds the plumbing required to talk to a Cadence
// frontend over gRPC. Unlike the Temporal Go SDK (which exposes a single
// client.Dial), the Cadence client is constructed from a YARPC dispatcher, a
// gRPC transport and a Thrift->Proto compatibility adapter. This package hides
// that wiring so the worker and runner commands can share it.
package cadenceclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/uber-go/tally"
	apiv1 "github.com/uber/cadence-idl/go/proto/api/v1"
	"go.uber.org/cadence/.gen/go/cadence/workflowserviceclient"
	"go.uber.org/cadence/client"
	"go.uber.org/cadence/compatibility"
	"go.uber.org/yarpc"
	"go.uber.org/yarpc/api/transport"
	"go.uber.org/yarpc/peer"
	"go.uber.org/yarpc/peer/hostport"
	"go.uber.org/yarpc/transport/grpc"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
)

const (
	// cadenceFrontendService is the YARPC service name that the Cadence
	// frontend advertises. It must match the server side.
	cadenceFrontendService = "cadence-frontend"
	// clientName is the YARPC caller name used for the dispatcher.
	clientName = "benchmark-workers-cadence"
)

// Config describes how to connect to a Cadence frontend.
type Config struct {
	// HostPort is the Cadence frontend gRPC endpoint (host:port).
	HostPort string
	// Domain is the Cadence domain (the equivalent of a Temporal namespace).
	Domain string
	// Identity, if set, is reported to the server to identify this client.
	Identity string

	// TLS configuration. TLS is enabled when both TLSCertPath and TLSKeyPath
	// are provided.
	TLSCertPath                string
	TLSKeyPath                 string
	TLSCAPath                  string
	TLSDisableHostVerification bool

	// PrometheusEndpoint, if set, enables a Prometheus metrics scope served on
	// this listen address. When empty a no-op scope is used.
	PrometheusEndpoint string
}

// Connection bundles the constructed Cadence client and the dependencies that
// the worker needs (service client, metrics scope, logger). Call Close to tear
// down the underlying dispatcher.
type Connection struct {
	Service workflowserviceclient.Interface
	Client  client.Client
	Scope   tally.Scope
	Logger  *zap.Logger

	dispatcher *yarpc.Dispatcher
}

// Dial constructs the Cadence service client and a high-level client.Client.
func Dial(cfg Config) (*Connection, error) {
	if cfg.HostPort == "" {
		return nil, fmt.Errorf("cadence endpoint not set (CADENCE_GRPC_ENDPOINT)")
	}

	scope := tally.NoopScope
	if cfg.PrometheusEndpoint != "" {
		s, err := newPrometheusScope(cfg.PrometheusEndpoint)
		if err != nil {
			return nil, err
		}
		scope = s
	}

	grpcTransport := grpc.NewTransport()

	var outbound transport.UnaryOutbound
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		tlsConfig, err := buildTLSConfig(cfg)
		if err != nil {
			return nil, err
		}
		creds := credentials.NewTLS(tlsConfig)
		chooser := peer.NewSingle(
			hostport.Identify(cfg.HostPort),
			grpcTransport.NewDialer(grpc.DialerCredentials(creds)),
		)
		outbound = grpcTransport.NewOutbound(chooser)
	} else {
		outbound = grpcTransport.NewSingleOutbound(cfg.HostPort)
	}

	dispatcher := yarpc.NewDispatcher(yarpc.Config{
		Name: clientName,
		Outbounds: yarpc.Outbounds{
			cadenceFrontendService: {Unary: outbound},
		},
	})
	if err := dispatcher.Start(); err != nil {
		return nil, fmt.Errorf("failed to start yarpc dispatcher: %w", err)
	}

	cc := dispatcher.ClientConfig(cadenceFrontendService)
	service := compatibility.NewThrift2ProtoAdapter(
		apiv1.NewDomainAPIYARPCClient(cc),
		apiv1.NewWorkflowAPIYARPCClient(cc),
		apiv1.NewWorkerAPIYARPCClient(cc),
		apiv1.NewVisibilityAPIYARPCClient(cc),
	)

	c := client.NewClient(service, cfg.Domain, &client.Options{
		Identity:     cfg.Identity,
		MetricsScope: scope,
	})

	return &Connection{
		Service:    service,
		Client:     c,
		Scope:      scope,
		Logger:     zap.NewNop(),
		dispatcher: dispatcher,
	}, nil
}

// Close stops the underlying YARPC dispatcher.
func (c *Connection) Close() {
	if c.dispatcher != nil {
		_ = c.dispatcher.Stop()
	}
}

func buildTLSConfig(cfg Config) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCertPath, cfg.TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load TLS key pair: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	if cfg.TLSCAPath != "" {
		caPool := x509.NewCertPool()
		b, err := os.ReadFile(cfg.TLSCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed reading server CA: %w", err)
		}
		if !caPool.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("server CA PEM file invalid")
		}
		tlsConfig.RootCAs = caPool
	}

	if cfg.TLSDisableHostVerification {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}
