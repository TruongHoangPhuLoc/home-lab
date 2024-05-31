package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"
	api_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
)

const (
	dynamicSSLReloadParam     = "ssl-dynamic-reload"
	dynamicWeightChangesParam = "weight-changes-dynamic-reload"
)

var (
	healthStatus = flag.Bool("health-status", false,
		`Add a location based on the value of health-status-uri to the default server. The location responds with the 200 status code for any request.
	Useful for external health-checking of the Ingress Controller`)

	healthStatusURI = flag.String("health-status-uri", "/nginx-health",
		`Sets the URI of health status location in the default server. Requires -health-status`)

	proxyURL = flag.String("proxy", "",
		`Use a proxy server to connect to Kubernetes API started by "kubectl proxy" command. For testing purposes only.
	The Ingress Controller does not start NGINX and does not write any generated NGINX configuration files to disk`)

	watchNamespace = flag.String("watch-namespace", api_v1.NamespaceAll,
		`Comma separated list of namespaces the Ingress Controller should watch for resources. By default the Ingress Controller watches all namespaces. Mutually exclusive with "watch-namespace-label".`)

	watchNamespaces []string

	watchSecretNamespace = flag.String("watch-secret-namespace", "",
		`Comma separated list of namespaces the Ingress Controller should watch for secrets. If this arg is not configured, the Ingress Controller watches the same namespaces for all resources. See "watch-namespace" and "watch-namespace-label". `)

	watchSecretNamespaces []string

	watchNamespaceLabel = flag.String("watch-namespace-label", "",
		`Configures the Ingress Controller to watch only those namespaces with label foo=bar. By default the Ingress Controller watches all namespaces. Mutually exclusive with "watch-namespace". `)

	nginxConfigMaps = flag.String("nginx-configmaps", "",
		`A ConfigMap resource for customizing NGINX configuration. If a ConfigMap is set,
	but the Ingress Controller is not able to fetch it from Kubernetes API, the Ingress Controller will fail to start.
	Format: <namespace>/<name>`)

	nginxPlus = flag.Bool("nginx-plus", false, "Enable support for NGINX Plus")

	appProtect = flag.Bool("enable-app-protect", false, "Enable support for NGINX App Protect. Requires -nginx-plus.")

	appProtectLogLevel = flag.String("app-protect-log-level", appProtectLogLevelDefault,
		`Sets log level for App Protect. Allowed values: fatal, error, warn, info, debug, trace. Requires -nginx-plus and -enable-app-protect.`)

	appProtectDos = flag.Bool("enable-app-protect-dos", false, "Enable support for NGINX App Protect dos. Requires -nginx-plus.")

	appProtectDosDebug = flag.Bool("app-protect-dos-debug", false, "Enable debugging for App Protect Dos. Requires -nginx-plus and -enable-app-protect-dos.")

	appProtectDosMaxDaemons = flag.Int("app-protect-dos-max-daemons", 0, "Max number of ADMD instances. Requires -nginx-plus and -enable-app-protect-dos.")
	appProtectDosMaxWorkers = flag.Int("app-protect-dos-max-workers", 0, "Max number of nginx processes to support. Requires -nginx-plus and -enable-app-protect-dos.")
	appProtectDosMemory     = flag.Int("app-protect-dos-memory", 0, "RAM memory size to consume in MB. Requires -nginx-plus and -enable-app-protect-dos.")

	agent              = flag.Bool("agent", false, "Enable NGINX Agent")
	agentInstanceGroup = flag.String("agent-instance-group", "nginx-ingress-controller", "Grouping used to associate NGINX Ingress Controller instances")

	ingressClass = flag.String("ingress-class", "nginx",
		`A class of the Ingress Controller.

	An IngressClass resource with the name equal to the class must be deployed. Otherwise, the Ingress Controller will fail to start.
	The Ingress Controller only processes resources that belong to its class - i.e. have the "ingressClassName" field resource equal to the class.

	The Ingress Controller processes all the VirtualServer/VirtualServerRoute/TransportServer resources that do not have the "ingressClassName" field for all versions of kubernetes.`)

	defaultServerSecret = flag.String("default-server-tls-secret", "",
		`A Secret with a TLS certificate and key for TLS termination of the default server. Format: <namespace>/<name>.
	If not set, than the certificate and key in the file "/etc/nginx/secrets/default" are used.
	If "/etc/nginx/secrets/default" doesn't exist, the Ingress Controller will configure NGINX to reject TLS connections to the default server.
	If a secret is set, but the Ingress Controller is not able to fetch it from Kubernetes API or it is not set and the Ingress Controller
	fails to read the file "/etc/nginx/secrets/default", the Ingress Controller will fail to start.`)

	versionFlag = flag.Bool("version", false, "Print the version, git-commit hash and build date and exit")

	mainTemplatePath = flag.String("main-template-path", "",
		`Path to the main NGINX configuration template. (default for NGINX "nginx.tmpl"; default for NGINX Plus "nginx-plus.tmpl")`)

	ingressTemplatePath = flag.String("ingress-template-path", "",
		`Path to the ingress NGINX configuration template for an ingress resource.
	(default for NGINX "nginx.ingress.tmpl"; default for NGINX Plus "nginx-plus.ingress.tmpl")`)

	virtualServerTemplatePath = flag.String("virtualserver-template-path", "",
		`Path to the VirtualServer NGINX configuration template for a VirtualServer resource.
	(default for NGINX "nginx.virtualserver.tmpl"; default for NGINX Plus "nginx-plus.virtualserver.tmpl")`)

	transportServerTemplatePath = flag.String("transportserver-template-path", "",
		`Path to the TransportServer NGINX configuration template for a TransportServer resource.
	(default for NGINX "nginx.transportserver.tmpl"; default for NGINX Plus "nginx-plus.transportserver.tmpl")`)

	externalService = flag.String("external-service", "",
		`Specifies the name of the service with the type LoadBalancer through which the Ingress Controller pods are exposed externally.
	The external address of the service is used when reporting the status of Ingress, VirtualServer and VirtualServerRoute resources. For Ingress resources only: Requires -report-ingress-status.`)

	ingressLink = flag.String("ingresslink", "",
		`Specifies the name of the IngressLink resource, which exposes the Ingress Controller pods via a BIG-IP system.
	The IP of the BIG-IP system is used when reporting the status of Ingress, VirtualServer and VirtualServerRoute resources. For Ingress resources only: Requires -report-ingress-status.`)

	reportIngressStatus = flag.Bool("report-ingress-status", false,
		"Updates the address field in the status of Ingress resources. Requires the -external-service or -ingresslink flag, or the 'external-status-address' key in the ConfigMap.")

	leaderElectionEnabled = flag.Bool("enable-leader-election", true,
		"Enable Leader election to avoid multiple replicas of the controller reporting the status of Ingress, VirtualServer and VirtualServerRoute resources -- only one replica will report status (default true). See -report-ingress-status flag.")

	leaderElectionLockName = flag.String("leader-election-lock-name", "nginx-ingress-leader-election",
		`Specifies the name of the ConfigMap, within the same namespace as the controller, used as the lock for leader election. Requires -enable-leader-election.`)

	nginxStatusAllowCIDRs = flag.String("nginx-status-allow-cidrs", "127.0.0.1,::1", `Add IP/CIDR blocks to the allow list for NGINX stub_status or the NGINX Plus API. Separate multiple IP/CIDR by commas.`)

	allowedCIDRs []string

	nginxStatusPort = flag.Int("nginx-status-port", 8080,
		"Set the port where the NGINX stub_status or the NGINX Plus API is exposed. [1024 - 65535]")

	nginxStatus = flag.Bool("nginx-status", true,
		"Enable the NGINX stub_status, or the NGINX Plus API.")

	nginxDebug = flag.Bool("nginx-debug", false,
		"Enable debugging for NGINX. Uses the nginx-debug binary. Requires 'error-log-level: debug' in the ConfigMap.")

	nginxReloadTimeout = flag.Int("nginx-reload-timeout", 60000,
		`The timeout in milliseconds which the Ingress Controller will wait for a successful NGINX reload after a change or at the initial start. (default 60000)`)

	wildcardTLSSecret = flag.String("wildcard-tls-secret", "",
		`A Secret with a TLS certificate and key for TLS termination of every Ingress/VirtualServer host for which TLS termination is enabled but the Secret is not specified.
		Format: <namespace>/<name>. If the argument is not set, for such Ingress/VirtualServer hosts NGINX will break any attempt to establish a TLS connection.
		If the argument is set, but the Ingress Controller is not able to fetch the Secret from Kubernetes API, the Ingress Controller will fail to start.`)

	enablePrometheusMetrics = flag.Bool("enable-prometheus-metrics", false,
		"Enable exposing NGINX or NGINX Plus metrics in the Prometheus format")

	prometheusTLSSecretName = flag.String("prometheus-tls-secret", "",
		`A Secret with a TLS certificate and key for TLS termination of the prometheus endpoint.`)

	prometheusMetricsListenPort = flag.Int("prometheus-metrics-listen-port", 9113,
		"Set the port where the Prometheus metrics are exposed. [1024 - 65535]")

	enableServiceInsight = flag.Bool("enable-service-insight", false,
		`Enable service insight for external load balancers. Requires -nginx-plus`)

	serviceInsightTLSSecretName = flag.String("service-insight-tls-secret", "",
		`A Secret with a TLS certificate and key for TLS termination of the service insight.`)

	serviceInsightListenPort = flag.Int("service-insight-listen-port", 9114,
		"Set the port where the Service Insight stats are exposed. Requires -nginx-plus. [1024 - 65535]")

	enableCustomResources = flag.Bool("enable-custom-resources", true,
		"Enable custom resources")

	enableOIDC = flag.Bool("enable-oidc", false,
		"Enable OIDC Policies.")

	enableSnippets = flag.Bool("enable-snippets", false,
		"Enable custom NGINX configuration snippets in Ingress, VirtualServer, VirtualServerRoute and TransportServer resources.")

	globalConfiguration = flag.String("global-configuration", "",
		`The namespace/name of the GlobalConfiguration resource for global configuration of the Ingress Controller. Requires -enable-custom-resources. Format: <namespace>/<name>`)

	enableTLSPassthrough = flag.Bool("enable-tls-passthrough", false,
		"Enable TLS Passthrough on default port 443. Requires -enable-custom-resources")

	tlsPassthroughPort = flag.Int("tls-passthrough-port", 443, "Set custom port for TLS Passthrough. [1024 - 65535]")

	spireAgentAddress = flag.String("spire-agent-address", "",
		`Specifies the address of the running Spire agent. Requires -nginx-plus and is for use with NGINX Service Mesh only. If the flag is set,
			but the Ingress Controller is not able to connect with the Spire Agent, the Ingress Controller will fail to start.`)

	enableInternalRoutes = flag.Bool("enable-internal-routes", false,
		`Enable support for internal routes with NGINX Service Mesh. Requires -spire-agent-address and -nginx-plus. Is for use with NGINX Service Mesh only.`)

	readyStatus = flag.Bool("ready-status", true, "Enables the readiness endpoint '/nginx-ready'. The endpoint returns a success code when NGINX has loaded all the config after the startup")

	readyStatusPort = flag.Int("ready-status-port", 8081, "Set the port where the readiness endpoint is exposed. [1024 - 65535]")

	enableLatencyMetrics = flag.Bool("enable-latency-metrics", false,
		"Enable collection of latency metrics for upstreams. Requires -enable-prometheus-metrics")

	enableCertManager = flag.Bool("enable-cert-manager", false,
		"Enable cert-manager controller for VirtualServer resources. Requires -enable-custom-resources")

	enableExternalDNS = flag.Bool("enable-external-dns", false,
		"Enable external-dns controller for VirtualServer resources. Requires -enable-custom-resources")

	includeYearInLogs = flag.Bool("include-year", false,
		"Option to include the year in the log header")

	disableIPV6 = flag.Bool("disable-ipv6", false,
		`Disable IPV6 listeners explicitly for nodes that do not support the IPV6 stack`)

	defaultHTTPListenerPort = flag.Int("default-http-listener-port", 80, "Sets a custom port for the HTTP NGINX `default_server`. [1024 - 65535]")

	defaultHTTPSListenerPort = flag.Int("default-https-listener-port", 443, "Sets a custom port for the HTTPS `default_server`. [1024 - 65535]")

	enableDynamicSSLReload = flag.Bool(dynamicSSLReloadParam, true, "Enable reloading of SSL Certificates without restarting the NGINX process.")

	enableTelemetryReporting = flag.Bool("enable-telemetry-reporting", true, "Enable gathering and reporting of product related telemetry.")

	enableDynamicWeightChangesReload = flag.Bool(dynamicWeightChangesParam, false, "Enable changing weights of split clients without reloading NGINX. Requires -nginx-plus")

	startupCheckFn func() error
)

//gocyclo:ignore
func parseFlags() {
	flag.Parse()

	if *versionFlag {
		os.Exit(0)
	}

	initialChecks()

	validateWatchedNamespaces()

	validationChecks()

	if *enableTLSPassthrough && !*enableCustomResources {
		glog.Fatal("enable-tls-passthrough flag requires -enable-custom-resources")
	}

	if *appProtect && !*nginxPlus {
		glog.Fatal("NGINX App Protect support is for NGINX Plus only")
	}

	if *appProtectLogLevel != appProtectLogLevelDefault && !*appProtect && !*nginxPlus {
		glog.Fatal("app-protect-log-level support is for NGINX Plus only and App Protect is enable")
	}

	if *appProtectDos && !*nginxPlus {
		glog.Fatal("NGINX App Protect Dos support is for NGINX Plus only")
	}

	if *appProtectDosDebug && !*appProtectDos && !*nginxPlus {
		glog.Fatal("NGINX App Protect Dos debug support is for NGINX Plus only and App Protect Dos is enable")
	}

	if *appProtectDosMaxDaemons != 0 && !*appProtectDos && !*nginxPlus {
		glog.Fatal("NGINX App Protect Dos max daemons support is for NGINX Plus only and App Protect Dos is enable")
	}

	if *appProtectDosMaxWorkers != 0 && !*appProtectDos && !*nginxPlus {
		glog.Fatal("NGINX App Protect Dos max workers support is for NGINX Plus and App Protect Dos is enable")
	}

	if *appProtectDosMemory != 0 && !*appProtectDos && !*nginxPlus {
		glog.Fatal("NGINX App Protect Dos memory support is for NGINX Plus and App Protect Dos is enable")
	}

	if *enableInternalRoutes && *spireAgentAddress == "" {
		glog.Fatal("enable-internal-routes flag requires spire-agent-address")
	}

	if *enableLatencyMetrics && !*enablePrometheusMetrics {
		glog.Warning("enable-latency-metrics flag requires enable-prometheus-metrics, latency metrics will not be collected")
		*enableLatencyMetrics = false
	}

	if *enableServiceInsight && !*nginxPlus {
		glog.Warning("enable-service-insight flag support is for NGINX Plus, service insight endpoint will not be exposed")
		*enableServiceInsight = false
	}

	if *enableCertManager && !*enableCustomResources {
		glog.Fatal("enable-cert-manager flag requires -enable-custom-resources")
	}

	if *enableExternalDNS && !*enableCustomResources {
		glog.Fatal("enable-external-dns flag requires -enable-custom-resources")
	}

	if *ingressLink != "" && *externalService != "" {
		glog.Fatal("ingresslink and external-service cannot both be set")
	}

	if *enableDynamicWeightChangesReload && !*nginxPlus {
		glog.Warning("weight-changes-dynamic-reload flag support is for NGINX Plus, Dynamic Weight Changes will not be enabled")
		*enableDynamicWeightChangesReload = false
	}

	if *agent && !*appProtect {
		glog.Fatal("NGINX Agent is used to enable the Security Monitoring dashboard and requires NGINX App Protect to be enabled")
	}
}

func initialChecks() {
	err := flag.Lookup("logtostderr").Value.Set("true")
	if err != nil {
		glog.Fatalf("Error setting logtostderr to true: %v", err)
	}

	err = flag.Lookup("include_year").Value.Set(strconv.FormatBool(*includeYearInLogs))
	if err != nil {
		glog.Fatalf("Error setting include_year flag: %v", err)
	}

	if startupCheckFn != nil {
		err := startupCheckFn()
		if err != nil {
			glog.Fatalf("Failed startup check: %v", err)
		}
		glog.Info("AWS startup check passed")
	}

	glog.Infof("Starting with flags: %+q", os.Args[1:])

	unparsed := flag.Args()
	if len(unparsed) > 0 {
		glog.Warningf("Ignoring unhandled arguments: %+q", unparsed)
	}
}

func validateWatchedNamespaces() {
	if *watchNamespace != "" && *watchNamespaceLabel != "" {
		glog.Fatal("watch-namespace and -watch-namespace-label are mutually exclusive")
	}

	watchNamespaces = strings.Split(*watchNamespace, ",")

	if *watchNamespace != "" {
		glog.Infof("Namespaces watched: %v", watchNamespaces)
		namespacesNameValidationError := validateNamespaceNames(watchNamespaces)
		if namespacesNameValidationError != nil {
			glog.Fatalf("Invalid values for namespaces: %v", namespacesNameValidationError)
		}
	}

	if len(*watchSecretNamespace) > 0 {
		watchSecretNamespaces = strings.Split(*watchSecretNamespace, ",")
		glog.Infof("Namespaces watched for secrets: %v", watchSecretNamespaces)
		namespacesNameValidationError := validateNamespaceNames(watchSecretNamespaces)
		if namespacesNameValidationError != nil {
			glog.Fatalf("Invalid values for secret namespaces: %v", namespacesNameValidationError)
		}
	} else {
		// empty => default to watched namespaces
		watchSecretNamespaces = watchNamespaces
	}

	if *watchNamespaceLabel != "" {
		var err error
		_, err = labels.Parse(*watchNamespaceLabel)
		if err != nil {
			glog.Fatalf("Unable to parse label %v for watch namespace label: %v", *watchNamespaceLabel, err)
		}
	}
}

// validationChecks checks the values for various flags
func validationChecks() {
	healthStatusURIValidationError := validateLocation(*healthStatusURI)
	if healthStatusURIValidationError != nil {
		glog.Fatalf("Invalid value for health-status-uri: %v", healthStatusURIValidationError)
	}

	statusLockNameValidationError := validateResourceName(*leaderElectionLockName)
	if statusLockNameValidationError != nil {
		glog.Fatalf("Invalid value for leader-election-lock-name: %v", statusLockNameValidationError)
	}

	statusPortValidationError := validatePort(*nginxStatusPort)
	if statusPortValidationError != nil {
		glog.Fatalf("Invalid value for nginx-status-port: %v", statusPortValidationError)
	}

	metricsPortValidationError := validatePort(*prometheusMetricsListenPort)
	if metricsPortValidationError != nil {
		glog.Fatalf("Invalid value for prometheus-metrics-listen-port: %v", metricsPortValidationError)
	}

	readyStatusPortValidationError := validatePort(*readyStatusPort)
	if readyStatusPortValidationError != nil {
		glog.Fatalf("Invalid value for ready-status-port: %v", readyStatusPortValidationError)
	}

	healthProbePortValidationError := validatePort(*serviceInsightListenPort)
	if healthProbePortValidationError != nil {
		glog.Fatalf("Invalid value for service-insight-listen-port: %v", metricsPortValidationError)
	}

	var err error
	allowedCIDRs, err = parseNginxStatusAllowCIDRs(*nginxStatusAllowCIDRs)
	if err != nil {
		glog.Fatalf(`Invalid value for nginx-status-allow-cidrs: %v`, err)
	}

	if *appProtectLogLevel != appProtectLogLevelDefault && *appProtect && *nginxPlus {
		logLevelValidationError := validateAppProtectLogLevel(*appProtectLogLevel)
		if logLevelValidationError != nil {
			glog.Fatalf("Invalid value for app-protect-log-level: %v", *appProtectLogLevel)
		}
	}
}

// validateNamespaceNames validates the namespaces are in the correct format
func validateNamespaceNames(namespaces []string) error {
	var allErrs []error

	for _, ns := range namespaces {
		if ns != "" {
			ns = strings.TrimSpace(ns)
			err := validateResourceName(ns)
			if err != nil {
				allErrs = append(allErrs, err)
				fmt.Printf("error %v ", err)
			}
		}
	}
	if len(allErrs) > 0 {
		return fmt.Errorf("errors validating namespaces: %v", allErrs)
	}
	return nil
}

// validateResourceName validates the name of a resource
func validateResourceName(name string) error {
	allErrs := validation.IsDNS1123Subdomain(name)
	if len(allErrs) > 0 {
		return fmt.Errorf("invalid resource name %v: %v", name, allErrs)
	}
	return nil
}

// validatePort makes sure a given port is inside the valid port range for its usage
func validatePort(port int) error {
	if port < 1024 || port > 65535 {
		return fmt.Errorf("port outside of valid port range [1024 - 65535]: %v", port)
	}
	return nil
}

const appProtectLogLevelDefault = "fatal"

// validateAppProtectLogLevel makes sure a given logLevel is one of the allowed values
func validateAppProtectLogLevel(logLevel string) error {
	switch strings.ToLower(logLevel) {
	case
		"fatal",
		"error",
		"warn",
		"info",
		"debug",
		"trace":
		return nil
	}
	return fmt.Errorf("invalid App Protect log level: %v", logLevel)
}

// parseNginxStatusAllowCIDRs converts a comma separated CIDR/IP address string into an array of CIDR/IP addresses.
// It returns an array of the valid CIDR/IP addresses or an error if given an invalid address.
func parseNginxStatusAllowCIDRs(input string) (cidrs []string, err error) {
	cidrsArray := strings.Split(input, ",")
	for _, cidr := range cidrsArray {
		trimmedCidr := strings.TrimSpace(cidr)
		err := validateCIDRorIP(trimmedCidr)
		if err != nil {
			return cidrs, err
		}
		cidrs = append(cidrs, trimmedCidr)
	}
	return cidrs, nil
}

// validateCIDRorIP makes sure a given string is either a valid CIDR block or IP address.
// It an error if it is not valid.
func validateCIDRorIP(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("invalid CIDR address: an empty string is an invalid CIDR block or IP address")
	}
	_, _, err := net.ParseCIDR(cidr)
	if err == nil {
		return nil
	}
	ip := net.ParseIP(cidr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %v", cidr)
	}
	return nil
}

const (
	locationFmt    = `/[^\s{};]*`
	locationErrMsg = "must start with / and must not include any whitespace character, `{`, `}` or `;`"
)

var locationRegexp = regexp.MustCompile("^" + locationFmt + "$")

func validateLocation(location string) error {
	if location == "" || location == "/" {
		return fmt.Errorf("invalid location format: '%v' is an invalid location", location)
	}
	if !locationRegexp.MatchString(location) {
		msg := validation.RegexError(locationErrMsg, locationFmt, "/path", "/path/subpath-123")
		return fmt.Errorf("invalid location format: %v", msg)
	}
	return nil
}
