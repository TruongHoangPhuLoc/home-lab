package configs

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/nginxinc/kubernetes-ingress/internal/configs/version2"
	"github.com/nginxinc/kubernetes-ingress/internal/k8s/secrets"
	"github.com/nginxinc/kubernetes-ingress/internal/nginx"
	conf_v1 "github.com/nginxinc/kubernetes-ingress/pkg/apis/configuration/v1"
	api_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	nginx502Server                                  = "unix:/var/lib/nginx/nginx-502-server.sock"
	internalLocationPrefix                          = "internal_location_"
	nginx418Server                                  = "unix:/var/lib/nginx/nginx-418-server.sock"
	specContext                                     = "spec"
	routeContext                                    = "route"
	subRouteContext                                 = "subroute"
	keyvalZoneBasePath                              = "/etc/nginx/state_files"
	splitClientsKeyValZoneSize                      = "100k"
	splitClientAmountWhenWeightChangesDynamicReload = 101
	defaultLogOutput                                = "syslog:server=localhost:514"
)

var grpcConflictingErrors = map[int]bool{
	400: true,
	401: true,
	403: true,
	404: true,
	405: true,
	408: true,
	413: true,
	414: true,
	415: true,
	426: true,
	429: true,
	495: true,
	496: true,
	497: true,
	500: true,
	501: true,
	502: true,
	503: true,
	504: true,
}

var incompatibleLBMethodsForSlowStart = map[string]bool{
	"random":                          true,
	"ip_hash":                         true,
	"random two":                      true,
	"random two least_conn":           true,
	"random two least_time=header":    true,
	"random two least_time=last_byte": true,
}

// MeshPodOwner contains the type and name of the K8s resource that owns the pod.
// This owner information is needed for NGINX Service Mesh metrics.
type MeshPodOwner struct {
	// OwnerType is one of the following: statefulset, daemonset, deployment.
	OwnerType string
	// OwnerName is the name of the statefulset, daemonset, or deployment.
	OwnerName string
}

// PodInfo contains the name of the Pod and the MeshPodOwner information
// which is used for NGINX Service Mesh metrics.
type PodInfo struct {
	Name string
	MeshPodOwner
}

// VirtualServerEx holds a VirtualServer along with the resources that are referenced in this VirtualServer.
type VirtualServerEx struct {
	VirtualServer       *conf_v1.VirtualServer
	HTTPPort            int
	HTTPSPort           int
	Endpoints           map[string][]string
	VirtualServerRoutes []*conf_v1.VirtualServerRoute
	ExternalNameSvcs    map[string]bool
	Policies            map[string]*conf_v1.Policy
	PodsByIP            map[string]PodInfo
	SecretRefs          map[string]*secrets.SecretReference
	ApPolRefs           map[string]*unstructured.Unstructured
	LogConfRefs         map[string]*unstructured.Unstructured
	DosProtectedRefs    map[string]*unstructured.Unstructured
	DosProtectedEx      map[string]*DosEx
}

func (vsx *VirtualServerEx) String() string {
	if vsx == nil {
		return "<nil>"
	}

	if vsx.VirtualServer == nil {
		return "VirtualServerEx has no VirtualServer"
	}

	return fmt.Sprintf("%s/%s", vsx.VirtualServer.Namespace, vsx.VirtualServer.Name)
}

// appProtectResourcesForVS holds file names of APPolicy and APLogConf resources used in a VirtualServer.
type appProtectResourcesForVS struct {
	Policies map[string]string
	LogConfs map[string]string
}

func newAppProtectVSResourcesForVS() *appProtectResourcesForVS {
	return &appProtectResourcesForVS{
		Policies: make(map[string]string),
		LogConfs: make(map[string]string),
	}
}

// GenerateEndpointsKey generates a key for the Endpoints map in VirtualServerEx.
func GenerateEndpointsKey(
	serviceNamespace string,
	serviceName string,
	subselector map[string]string,
	port uint16,
) string {
	if len(subselector) > 0 {
		return fmt.Sprintf("%s/%s_%s:%d", serviceNamespace, serviceName, labels.Set(subselector).String(), port)
	}
	return fmt.Sprintf("%s/%s:%d", serviceNamespace, serviceName, port)
}

type upstreamNamer struct {
	prefix    string
	namespace string
}

// NewUpstreamNamerForVirtualServer creates a new namer.
//
//nolint:revive
func NewUpstreamNamerForVirtualServer(virtualServer *conf_v1.VirtualServer) *upstreamNamer {
	return &upstreamNamer{
		prefix:    fmt.Sprintf("vs_%s_%s", virtualServer.Namespace, virtualServer.Name),
		namespace: virtualServer.Namespace,
	}
}

// NewUpstreamNamerForVirtualServerRoute creates a new namer.
//
//nolint:revive
func NewUpstreamNamerForVirtualServerRoute(virtualServer *conf_v1.VirtualServer, virtualServerRoute *conf_v1.VirtualServerRoute) *upstreamNamer {
	return &upstreamNamer{
		prefix: fmt.Sprintf(
			"vs_%s_%s_vsr_%s_%s",
			virtualServer.Namespace,
			virtualServer.Name,
			virtualServerRoute.Namespace,
			virtualServerRoute.Name,
		),
		namespace: virtualServerRoute.Namespace,
	}
}

func (namer *upstreamNamer) GetNameForUpstreamFromAction(action *conf_v1.Action) string {
	var upstream string
	if action.Proxy != nil && action.Proxy.Upstream != "" {
		upstream = action.Proxy.Upstream
	} else {
		upstream = action.Pass
	}

	return fmt.Sprintf("%s_%s", namer.prefix, upstream)
}

func (namer *upstreamNamer) GetNameForUpstream(upstream string) string {
	return fmt.Sprintf("%s_%s", namer.prefix, upstream)
}

// VariableNamer is a namer which generates unique variable names for a VirtualServer.
type VariableNamer struct {
	safeNsName string
}

// NewVSVariableNamer creates a new namer for a VirtualServer.
func NewVSVariableNamer(virtualServer *conf_v1.VirtualServer) *VariableNamer {
	safeNsName := strings.ReplaceAll(fmt.Sprintf("%s_%s", virtualServer.Namespace, virtualServer.Name), "-", "_")
	return &VariableNamer{
		safeNsName: safeNsName,
	}
}

// GetNameOfKeyvalZoneForSplitClientIndex returns a unique name for a keyval zone for split clients.
func (namer *VariableNamer) GetNameOfKeyvalZoneForSplitClientIndex(index int) string {
	return fmt.Sprintf("vs_%s_keyval_zone_split_clients_%d", namer.safeNsName, index)
}

// GetNameOfKeyvalForSplitClientIndex returns a unique name for a keyval for split clients.
func (namer *VariableNamer) GetNameOfKeyvalForSplitClientIndex(index int) string {
	return fmt.Sprintf("$vs_%s_keyval_split_clients_%d", namer.safeNsName, index)
}

// GetNameOfKeyvalKeyForSplitClientIndex returns a unique name for a keyval key for split clients.
func (namer *VariableNamer) GetNameOfKeyvalKeyForSplitClientIndex(index int) string {
	return fmt.Sprintf("\"vs_%s_keyval_key_split_clients_%d\"", namer.safeNsName, index)
}

// GetNameOfMapForSplitClientIndex returns a unique name for a map for split clients.
func (namer *VariableNamer) GetNameOfMapForSplitClientIndex(index int) string {
	return fmt.Sprintf("$vs_%s_map_split_clients_%d", namer.safeNsName, index)
}

// GetNameOfKeyOfMapForWeights returns a unique name for a key of a map for split clients.
func (namer *VariableNamer) GetNameOfKeyOfMapForWeights(index int, i int, j int) string {
	return fmt.Sprintf("\"vs_%s_split_clients_%d_%d_%d\"", namer.safeNsName, index, i, j)
}

// GetNameOfSplitClientsForWeights gets the name of the split clients for a particular combination of weights and scIndex.
func (namer *VariableNamer) GetNameOfSplitClientsForWeights(index int, i int, j int) string {
	return fmt.Sprintf("$vs_%s_split_clients_%d_%d_%d", namer.safeNsName, index, i, j)
}

// GetNameForSplitClientVariable gets the name of a split client variable for a particular scIndex.
func (namer *VariableNamer) GetNameForSplitClientVariable(index int) string {
	return fmt.Sprintf("$vs_%s_splits_%d", namer.safeNsName, index)
}

// GetNameForVariableForMatchesRouteMap gets the name of a matches route map
func (namer *VariableNamer) GetNameForVariableForMatchesRouteMap(
	matchesIndex int,
	matchIndex int,
	conditionIndex int,
) string {
	return fmt.Sprintf("$vs_%s_matches_%d_match_%d_cond_%d", namer.safeNsName, matchesIndex, matchIndex, conditionIndex)
}

// GetNameForVariableForMatchesRouteMainMap gets the name of a matches route main map
func (namer *VariableNamer) GetNameForVariableForMatchesRouteMainMap(matchesIndex int) string {
	return fmt.Sprintf("$vs_%s_matches_%d", namer.safeNsName, matchesIndex)
}

func newHealthCheckWithDefaults(upstream conf_v1.Upstream, upstreamName string, cfgParams *ConfigParams) *version2.HealthCheck {
	uri := "/"
	if isGRPC(upstream.Type) {
		uri = ""
	}

	return &version2.HealthCheck{
		Name:                upstreamName,
		URI:                 uri,
		Interval:            "5s",
		Jitter:              "0s",
		KeepaliveTime:       "60s",
		Fails:               1,
		Passes:              1,
		ProxyPass:           fmt.Sprintf("%v://%v", generateProxyPassProtocol(upstream.TLS.Enable), upstreamName),
		ProxyConnectTimeout: generateTimeWithDefault(upstream.ProxyConnectTimeout, cfgParams.ProxyConnectTimeout),
		ProxyReadTimeout:    generateTimeWithDefault(upstream.ProxyReadTimeout, cfgParams.ProxyReadTimeout),
		ProxySendTimeout:    generateTimeWithDefault(upstream.ProxySendTimeout, cfgParams.ProxySendTimeout),
		Headers:             make(map[string]string),
		GRPCPass:            generateGRPCPass(isGRPC(upstream.Type), upstream.TLS.Enable, upstreamName),
	}
}

// VirtualServerConfigurator generates a VirtualServer configuration
type virtualServerConfigurator struct {
	cfgParams                  *ConfigParams
	isPlus                     bool
	isWildcardEnabled          bool
	isResolverConfigured       bool
	isTLSPassthrough           bool
	enableSnippets             bool
	warnings                   Warnings
	spiffeCerts                bool
	enableInternalRoutes       bool
	oidcPolCfg                 *oidcPolicyCfg
	isIPV6Disabled             bool
	DynamicSSLReloadEnabled    bool
	StaticSSLPath              string
	DynamicWeightChangesReload bool
	bundleValidator            bundleValidator
}

type oidcPolicyCfg struct {
	oidc *version2.OIDC
	key  string
}

func (vsc *virtualServerConfigurator) addWarningf(obj runtime.Object, msgFmt string, args ...interface{}) {
	vsc.warnings.AddWarningf(obj, msgFmt, args...)
}

func (vsc *virtualServerConfigurator) addWarnings(obj runtime.Object, msgs []string) {
	for _, msg := range msgs {
		vsc.warnings.AddWarning(obj, msg)
	}
}

func (vsc *virtualServerConfigurator) clearWarnings() {
	vsc.warnings = make(map[runtime.Object][]string)
}

// newVirtualServerConfigurator creates a new VirtualServerConfigurator
func newVirtualServerConfigurator(
	cfgParams *ConfigParams,
	isPlus bool,
	isResolverConfigured bool,
	staticParams *StaticConfigParams,
	isWildcardEnabled bool,
	bundleValidator bundleValidator,
) *virtualServerConfigurator {
	if bundleValidator == nil {
		bundleValidator = newInternalBundleValidator(appProtectBundleFolder)
	}
	return &virtualServerConfigurator{
		cfgParams:                  cfgParams,
		isPlus:                     isPlus,
		isWildcardEnabled:          isWildcardEnabled,
		isResolverConfigured:       isResolverConfigured,
		isTLSPassthrough:           staticParams.TLSPassthrough,
		enableSnippets:             staticParams.EnableSnippets,
		warnings:                   make(map[runtime.Object][]string),
		spiffeCerts:                staticParams.NginxServiceMesh,
		enableInternalRoutes:       staticParams.EnableInternalRoutes,
		oidcPolCfg:                 &oidcPolicyCfg{},
		isIPV6Disabled:             staticParams.DisableIPV6,
		DynamicSSLReloadEnabled:    staticParams.DynamicSSLReload,
		StaticSSLPath:              staticParams.StaticSSLPath,
		DynamicWeightChangesReload: staticParams.DynamicWeightChangesReload,
		bundleValidator:            bundleValidator,
	}
}

func (vsc *virtualServerConfigurator) generateEndpointsForUpstream(
	owner runtime.Object,
	namespace string,
	upstream conf_v1.Upstream,
	virtualServerEx *VirtualServerEx,
) []string {
	endpointsKey := GenerateEndpointsKey(namespace, upstream.Service, upstream.Subselector, upstream.Port)
	externalNameSvcKey := GenerateExternalNameSvcKey(namespace, upstream.Service)
	endpoints := virtualServerEx.Endpoints[endpointsKey]
	if !vsc.isPlus && len(endpoints) == 0 {
		return []string{nginx502Server}
	}

	_, isExternalNameSvc := virtualServerEx.ExternalNameSvcs[externalNameSvcKey]
	if isExternalNameSvc && !vsc.isResolverConfigured {
		msgFmt := "Type ExternalName service %v in upstream %v will be ignored. To use ExternaName services, a resolver must be configured in the ConfigMap"
		vsc.addWarningf(owner, msgFmt, upstream.Service, upstream.Name)
		endpoints = []string{}
	}

	return endpoints
}

func (vsc *virtualServerConfigurator) generateBackupEndpointsForUpstream(
	owner runtime.Object,
	namespace string,
	upstream conf_v1.Upstream,
	virtualServerEx *VirtualServerEx,
) []string {
	if upstream.Backup == "" || upstream.BackupPort == nil {
		return []string{}
	}
	externalNameSvcKey := GenerateExternalNameSvcKey(namespace, upstream.Backup)
	_, isExternalNameSvc := virtualServerEx.ExternalNameSvcs[externalNameSvcKey]
	if isExternalNameSvc && !vsc.isResolverConfigured {
		msgFmt := "Type ExternalName service %v in upstream %v will be ignored. To use ExternaName services, a resolver must be configured in the ConfigMap"
		vsc.addWarningf(owner, msgFmt, upstream.Backup, upstream.Name)
		return []string{}
	}

	backupEndpointsKey := GenerateEndpointsKey(namespace, upstream.Backup, upstream.Subselector, *upstream.BackupPort)
	backupEndpoints := virtualServerEx.Endpoints[backupEndpointsKey]
	if len(backupEndpoints) == 0 {
		return []string{}
	}
	return backupEndpoints
}

// GenerateVirtualServerConfig generates a full configuration for a VirtualServer
func (vsc *virtualServerConfigurator) GenerateVirtualServerConfig(
	vsEx *VirtualServerEx,
	apResources *appProtectResourcesForVS,
	dosResources map[string]*appProtectDosResource,
) (version2.VirtualServerConfig, Warnings) {
	vsc.clearWarnings()

	useCustomListeners := false

	if vsEx.VirtualServer.Spec.Listener != nil {
		useCustomListeners = true
	}

	sslConfig := vsc.generateSSLConfig(vsEx.VirtualServer, vsEx.VirtualServer.Spec.TLS, vsEx.VirtualServer.Namespace, vsEx.SecretRefs, vsc.cfgParams)
	tlsRedirectConfig := generateTLSRedirectConfig(vsEx.VirtualServer.Spec.TLS)

	policyOpts := policyOptions{
		tls:         sslConfig != nil,
		secretRefs:  vsEx.SecretRefs,
		apResources: apResources,
	}

	ownerDetails := policyOwnerDetails{
		owner:          vsEx.VirtualServer,
		ownerNamespace: vsEx.VirtualServer.Namespace,
		vsNamespace:    vsEx.VirtualServer.Namespace,
		vsName:         vsEx.VirtualServer.Name,
	}
	policiesCfg := vsc.generatePolicies(ownerDetails, vsEx.VirtualServer.Spec.Policies, vsEx.Policies, specContext, policyOpts)

	if policiesCfg.JWKSAuthEnabled {
		jwtAuthKey := policiesCfg.JWTAuth.Key
		policiesCfg.JWTAuthList = make(map[string]*version2.JWTAuth)
		policiesCfg.JWTAuthList[jwtAuthKey] = policiesCfg.JWTAuth
	}

	dosCfg := generateDosCfg(dosResources[""])

	// enabledInternalRoutes controls if a virtual server is configured as an internal route.
	enabledInternalRoutes := vsEx.VirtualServer.Spec.InternalRoute
	if vsEx.VirtualServer.Spec.InternalRoute && !vsc.enableInternalRoutes {
		vsc.addWarningf(vsEx.VirtualServer, "Internal Route cannot be configured for virtual server %s. Internal Routes can be enabled by setting the enable-internal-routes flag", vsEx.VirtualServer.Name)
		enabledInternalRoutes = false
	}

	// crUpstreams maps an UpstreamName to its conf_v1.Upstream as they are generated
	// necessary for generateLocation to know what Upstream each Location references
	crUpstreams := make(map[string]conf_v1.Upstream)

	virtualServerUpstreamNamer := NewUpstreamNamerForVirtualServer(vsEx.VirtualServer)
	var upstreams []version2.Upstream
	var statusMatches []version2.StatusMatch
	var healthChecks []version2.HealthCheck
	var limitReqZones []version2.LimitReqZone

	limitReqZones = append(limitReqZones, policiesCfg.LimitReqZones...)

	// generate upstreams for VirtualServer
	for _, u := range vsEx.VirtualServer.Spec.Upstreams {

		if (sslConfig == nil || !vsc.cfgParams.HTTP2) && isGRPC(u.Type) {
			vsc.addWarningf(vsEx.VirtualServer, "gRPC cannot be configured for upstream %s. gRPC requires enabled HTTP/2 and TLS termination.", u.Name)
		}

		upstreamName := virtualServerUpstreamNamer.GetNameForUpstream(u.Name)
		upstreamNamespace := vsEx.VirtualServer.Namespace
		endpoints := vsc.generateEndpointsForUpstream(vsEx.VirtualServer, upstreamNamespace, u, vsEx)
		backupEndpoints := vsc.generateBackupEndpointsForUpstream(vsEx.VirtualServer, upstreamNamespace, u, vsEx)

		// isExternalNameSvc is always false for OSS
		_, isExternalNameSvc := vsEx.ExternalNameSvcs[GenerateExternalNameSvcKey(upstreamNamespace, u.Service)]
		ups := vsc.generateUpstream(vsEx.VirtualServer, upstreamName, u, isExternalNameSvc, endpoints, backupEndpoints)
		upstreams = append(upstreams, ups)

		u.TLS.Enable = isTLSEnabled(u, vsc.spiffeCerts, vsEx.VirtualServer.Spec.InternalRoute)
		crUpstreams[upstreamName] = u

		if hc := generateHealthCheck(u, upstreamName, vsc.cfgParams); hc != nil {
			healthChecks = append(healthChecks, *hc)
			if u.HealthCheck.StatusMatch != "" {
				statusMatches = append(
					statusMatches,
					generateUpstreamStatusMatch(upstreamName, u.HealthCheck.StatusMatch),
				)
			}
		}
	}
	// generate upstreams for each VirtualServerRoute
	for _, vsr := range vsEx.VirtualServerRoutes {
		upstreamNamer := NewUpstreamNamerForVirtualServerRoute(vsEx.VirtualServer, vsr)
		for _, u := range vsr.Spec.Upstreams {
			if (sslConfig == nil || !vsc.cfgParams.HTTP2) && isGRPC(u.Type) {
				vsc.addWarningf(vsr, "gRPC cannot be configured for upstream %s. gRPC requires enabled HTTP/2 and TLS termination", u.Name)
			}

			upstreamName := upstreamNamer.GetNameForUpstream(u.Name)
			upstreamNamespace := vsr.Namespace
			endpoints := vsc.generateEndpointsForUpstream(vsr, upstreamNamespace, u, vsEx)
			backup := vsc.generateBackupEndpointsForUpstream(vsEx.VirtualServer, upstreamNamespace, u, vsEx)

			// isExternalNameSvc is always false for OSS
			_, isExternalNameSvc := vsEx.ExternalNameSvcs[GenerateExternalNameSvcKey(upstreamNamespace, u.Service)]
			ups := vsc.generateUpstream(vsr, upstreamName, u, isExternalNameSvc, endpoints, backup)
			upstreams = append(upstreams, ups)
			u.TLS.Enable = isTLSEnabled(u, vsc.spiffeCerts, vsEx.VirtualServer.Spec.InternalRoute)
			crUpstreams[upstreamName] = u

			if hc := generateHealthCheck(u, upstreamName, vsc.cfgParams); hc != nil {
				healthChecks = append(healthChecks, *hc)
				if u.HealthCheck.StatusMatch != "" {
					statusMatches = append(
						statusMatches,
						generateUpstreamStatusMatch(upstreamName, u.HealthCheck.StatusMatch),
					)
				}
			}
		}
	}

	var locations []version2.Location
	var internalRedirectLocations []version2.InternalRedirectLocation
	var returnLocations []version2.ReturnLocation
	var splitClients []version2.SplitClient
	var maps []version2.Map
	var errorPageLocations []version2.ErrorPageLocation
	var keyValZones []version2.KeyValZone
	var keyVals []version2.KeyVal
	var twoWaySplitClients []version2.TwoWaySplitClients
	vsrErrorPagesFromVs := make(map[string][]conf_v1.ErrorPage)
	vsrErrorPagesRouteIndex := make(map[string]int)
	vsrLocationSnippetsFromVs := make(map[string]string)
	vsrPoliciesFromVs := make(map[string][]conf_v1.PolicyReference)
	isVSR := false
	matchesRoutes := 0

	VariableNamer := NewVSVariableNamer(vsEx.VirtualServer)

	// generates config for VirtualServer routes
	for _, r := range vsEx.VirtualServer.Spec.Routes {
		errorPages := errorPageDetails{
			pages: r.ErrorPages,
			index: len(errorPageLocations),
			owner: vsEx.VirtualServer,
		}
		errorPageLocations = append(errorPageLocations, generateErrorPageLocations(errorPages.index, errorPages.pages)...)

		// ignore routes that reference VirtualServerRoute
		if r.Route != "" {
			name := r.Route
			if !strings.Contains(name, "/") {
				name = fmt.Sprintf("%v/%v", vsEx.VirtualServer.Namespace, r.Route)
			}

			// store route location snippet for the referenced VirtualServerRoute in case they don't define their own
			if r.LocationSnippets != "" {
				vsrLocationSnippetsFromVs[name] = r.LocationSnippets
			}

			// store route error pages and route index for the referenced VirtualServerRoute in case they don't define their own
			if len(r.ErrorPages) > 0 {
				vsrErrorPagesFromVs[name] = errorPages.pages
				vsrErrorPagesRouteIndex[name] = errorPages.index
			}

			// store route policies for the referenced VirtualServerRoute in case they don't define their own
			if len(r.Policies) > 0 {
				vsrPoliciesFromVs[name] = r.Policies
			}

			continue
		}

		vsLocSnippets := r.LocationSnippets
		ownerDetails := policyOwnerDetails{
			owner:          vsEx.VirtualServer,
			ownerNamespace: vsEx.VirtualServer.Namespace,
			vsNamespace:    vsEx.VirtualServer.Namespace,
			vsName:         vsEx.VirtualServer.Name,
		}
		routePoliciesCfg := vsc.generatePolicies(ownerDetails, r.Policies, vsEx.Policies, routeContext, policyOpts)
		if policiesCfg.OIDC {
			routePoliciesCfg.OIDC = policiesCfg.OIDC
		}
		if routePoliciesCfg.JWKSAuthEnabled {
			policiesCfg.JWKSAuthEnabled = routePoliciesCfg.JWKSAuthEnabled

			if policiesCfg.JWTAuthList == nil {
				policiesCfg.JWTAuthList = make(map[string]*version2.JWTAuth)
			}

			jwtAuthKey := routePoliciesCfg.JWTAuth.Key
			if _, exists := policiesCfg.JWTAuthList[jwtAuthKey]; !exists {
				policiesCfg.JWTAuthList[jwtAuthKey] = routePoliciesCfg.JWTAuth
			}
		}
		limitReqZones = append(limitReqZones, routePoliciesCfg.LimitReqZones...)

		dosRouteCfg := generateDosCfg(dosResources[r.Path])

		if len(r.Matches) > 0 {
			cfg := generateMatchesConfig(
				r,
				virtualServerUpstreamNamer,
				crUpstreams,
				VariableNamer,
				matchesRoutes,
				len(splitClients),
				vsc.cfgParams,
				errorPages,
				vsLocSnippets,
				vsc.enableSnippets,
				len(returnLocations),
				isVSR,
				"", "",
				vsc.warnings,
				vsc.DynamicWeightChangesReload,
			)
			addPoliciesCfgToLocations(routePoliciesCfg, cfg.Locations)
			addDosConfigToLocations(dosRouteCfg, cfg.Locations)

			maps = append(maps, cfg.Maps...)
			locations = append(locations, cfg.Locations...)
			internalRedirectLocations = append(internalRedirectLocations, cfg.InternalRedirectLocation)
			returnLocations = append(returnLocations, cfg.ReturnLocations...)
			splitClients = append(splitClients, cfg.SplitClients...)
			keyValZones = append(keyValZones, cfg.KeyValZones...)
			keyVals = append(keyVals, cfg.KeyVals...)
			twoWaySplitClients = append(twoWaySplitClients, cfg.TwoWaySplitClients...)
			matchesRoutes++
		} else if len(r.Splits) > 0 {
			cfg := generateDefaultSplitsConfig(r, virtualServerUpstreamNamer, crUpstreams, VariableNamer, len(splitClients),
				vsc.cfgParams, errorPages, r.Path, vsLocSnippets, vsc.enableSnippets, len(returnLocations), isVSR, "", "", vsc.warnings, vsc.DynamicWeightChangesReload)
			addPoliciesCfgToLocations(routePoliciesCfg, cfg.Locations)
			addDosConfigToLocations(dosRouteCfg, cfg.Locations)
			splitClients = append(splitClients, cfg.SplitClients...)
			locations = append(locations, cfg.Locations...)
			internalRedirectLocations = append(internalRedirectLocations, cfg.InternalRedirectLocation)
			returnLocations = append(returnLocations, cfg.ReturnLocations...)
			maps = append(maps, cfg.Maps...)
			keyValZones = append(keyValZones, cfg.KeyValZones...)
			keyVals = append(keyVals, cfg.KeyVals...)
			twoWaySplitClients = append(twoWaySplitClients, cfg.TwoWaySplitClients...)
		} else {
			upstreamName := virtualServerUpstreamNamer.GetNameForUpstreamFromAction(r.Action)
			upstream := crUpstreams[upstreamName]

			proxySSLName := generateProxySSLName(upstream.Service, vsEx.VirtualServer.Namespace)

			loc, returnLoc := generateLocation(r.Path, upstreamName, upstream, r.Action, vsc.cfgParams, errorPages, false,
				proxySSLName, r.Path, vsLocSnippets, vsc.enableSnippets, len(returnLocations), isVSR, "", "", vsc.warnings)
			addPoliciesCfgToLocation(routePoliciesCfg, &loc)
			loc.Dos = dosRouteCfg

			locations = append(locations, loc)
			if returnLoc != nil {
				returnLocations = append(returnLocations, *returnLoc)
			}
		}
	}

	// generate config for subroutes of each VirtualServerRoute
	for _, vsr := range vsEx.VirtualServerRoutes {
		isVSR := true
		upstreamNamer := NewUpstreamNamerForVirtualServerRoute(vsEx.VirtualServer, vsr)
		for _, r := range vsr.Spec.Subroutes {
			errorPages := errorPageDetails{
				pages: r.ErrorPages,
				index: len(errorPageLocations),
				owner: vsr,
			}
			errorPageLocations = append(errorPageLocations, generateErrorPageLocations(errorPages.index, errorPages.pages)...)
			vsrNamespaceName := fmt.Sprintf("%v/%v", vsr.Namespace, vsr.Name)
			// use the VirtualServer error pages if the route does not define any
			if r.ErrorPages == nil {
				if vsErrorPages, ok := vsrErrorPagesFromVs[vsrNamespaceName]; ok {
					errorPages.pages = vsErrorPages
					errorPages.index = vsrErrorPagesRouteIndex[vsrNamespaceName]
				}
			}

			locSnippets := r.LocationSnippets
			// use the  VirtualServer location snippet if the route does not define any
			if r.LocationSnippets == "" {
				locSnippets = vsrLocationSnippetsFromVs[vsrNamespaceName]
			}

			var ownerDetails policyOwnerDetails
			var policyRefs []conf_v1.PolicyReference
			var context string
			if len(r.Policies) == 0 {
				// use the VirtualServer route policies if the route does not define any
				ownerDetails = policyOwnerDetails{
					owner:          vsEx.VirtualServer,
					ownerNamespace: vsEx.VirtualServer.Namespace,
					vsNamespace:    vsEx.VirtualServer.Namespace,
					vsName:         vsEx.VirtualServer.Name,
				}
				policyRefs = vsrPoliciesFromVs[vsrNamespaceName]
				context = routeContext
			} else {
				ownerDetails = policyOwnerDetails{
					owner:          vsr,
					ownerNamespace: vsr.Namespace,
					vsNamespace:    vsEx.VirtualServer.Namespace,
					vsName:         vsEx.VirtualServer.Name,
				}
				policyRefs = r.Policies
				context = subRouteContext
			}
			routePoliciesCfg := vsc.generatePolicies(ownerDetails, policyRefs, vsEx.Policies, context, policyOpts)
			if policiesCfg.OIDC {
				routePoliciesCfg.OIDC = policiesCfg.OIDC
			}
			if routePoliciesCfg.JWKSAuthEnabled {
				policiesCfg.JWKSAuthEnabled = routePoliciesCfg.JWKSAuthEnabled

				if policiesCfg.JWTAuthList == nil {
					policiesCfg.JWTAuthList = make(map[string]*version2.JWTAuth)
				}

				jwtAuthKey := routePoliciesCfg.JWTAuth.Key
				if _, exists := policiesCfg.JWTAuthList[jwtAuthKey]; !exists {
					policiesCfg.JWTAuthList[jwtAuthKey] = routePoliciesCfg.JWTAuth
				}
			}
			limitReqZones = append(limitReqZones, routePoliciesCfg.LimitReqZones...)

			dosRouteCfg := generateDosCfg(dosResources[r.Path])

			if len(r.Matches) > 0 {
				cfg := generateMatchesConfig(
					r,
					upstreamNamer,
					crUpstreams,
					VariableNamer,
					matchesRoutes,
					len(splitClients),
					vsc.cfgParams,
					errorPages,
					locSnippets,
					vsc.enableSnippets,
					len(returnLocations),
					isVSR,
					vsr.Name,
					vsr.Namespace,
					vsc.warnings,
					vsc.DynamicWeightChangesReload,
				)
				addPoliciesCfgToLocations(routePoliciesCfg, cfg.Locations)
				addDosConfigToLocations(dosRouteCfg, cfg.Locations)

				maps = append(maps, cfg.Maps...)
				locations = append(locations, cfg.Locations...)
				internalRedirectLocations = append(internalRedirectLocations, cfg.InternalRedirectLocation)
				returnLocations = append(returnLocations, cfg.ReturnLocations...)
				splitClients = append(splitClients, cfg.SplitClients...)
				keyValZones = append(keyValZones, cfg.KeyValZones...)
				keyVals = append(keyVals, cfg.KeyVals...)
				twoWaySplitClients = append(twoWaySplitClients, cfg.TwoWaySplitClients...)
				matchesRoutes++
			} else if len(r.Splits) > 0 {
				cfg := generateDefaultSplitsConfig(r, upstreamNamer, crUpstreams, VariableNamer, len(splitClients), vsc.cfgParams,
					errorPages, r.Path, locSnippets, vsc.enableSnippets, len(returnLocations), isVSR, vsr.Name, vsr.Namespace, vsc.warnings, vsc.DynamicWeightChangesReload)
				addPoliciesCfgToLocations(routePoliciesCfg, cfg.Locations)
				addDosConfigToLocations(dosRouteCfg, cfg.Locations)

				splitClients = append(splitClients, cfg.SplitClients...)
				locations = append(locations, cfg.Locations...)
				internalRedirectLocations = append(internalRedirectLocations, cfg.InternalRedirectLocation)
				returnLocations = append(returnLocations, cfg.ReturnLocations...)
				keyValZones = append(keyValZones, cfg.KeyValZones...)
				keyVals = append(keyVals, cfg.KeyVals...)
				twoWaySplitClients = append(twoWaySplitClients, cfg.TwoWaySplitClients...)
				maps = append(maps, cfg.Maps...)
			} else {
				upstreamName := upstreamNamer.GetNameForUpstreamFromAction(r.Action)
				upstream := crUpstreams[upstreamName]
				proxySSLName := generateProxySSLName(upstream.Service, vsr.Namespace)

				loc, returnLoc := generateLocation(r.Path, upstreamName, upstream, r.Action, vsc.cfgParams, errorPages, false,
					proxySSLName, r.Path, locSnippets, vsc.enableSnippets, len(returnLocations), isVSR, vsr.Name, vsr.Namespace, vsc.warnings)
				addPoliciesCfgToLocation(routePoliciesCfg, &loc)
				loc.Dos = dosRouteCfg

				locations = append(locations, loc)
				if returnLoc != nil {
					returnLocations = append(returnLocations, *returnLoc)
				}
			}
		}
	}

	httpSnippets := generateSnippets(vsc.enableSnippets, vsEx.VirtualServer.Spec.HTTPSnippets, []string{})
	serverSnippets := generateSnippets(
		vsc.enableSnippets,
		vsEx.VirtualServer.Spec.ServerSnippets,
		vsc.cfgParams.ServerSnippets,
	)

	sort.Slice(upstreams, func(i, j int) bool {
		return upstreams[i].Name < upstreams[j].Name
	})

	vsCfg := version2.VirtualServerConfig{
		Upstreams:     upstreams,
		SplitClients:  splitClients,
		Maps:          maps,
		StatusMatches: statusMatches,
		LimitReqZones: removeDuplicateLimitReqZones(limitReqZones),
		HTTPSnippets:  httpSnippets,
		Server: version2.Server{
			ServerName:                vsEx.VirtualServer.Spec.Host,
			Gunzip:                    vsEx.VirtualServer.Spec.Gunzip,
			StatusZone:                vsEx.VirtualServer.Spec.Host,
			HTTPPort:                  vsEx.HTTPPort,
			HTTPSPort:                 vsEx.HTTPSPort,
			CustomListeners:           useCustomListeners,
			ProxyProtocol:             vsc.cfgParams.ProxyProtocol,
			SSL:                       sslConfig,
			ServerTokens:              vsc.cfgParams.ServerTokens,
			SetRealIPFrom:             vsc.cfgParams.SetRealIPFrom,
			RealIPHeader:              vsc.cfgParams.RealIPHeader,
			RealIPRecursive:           vsc.cfgParams.RealIPRecursive,
			Snippets:                  serverSnippets,
			InternalRedirectLocations: internalRedirectLocations,
			Locations:                 locations,
			ReturnLocations:           returnLocations,
			HealthChecks:              healthChecks,
			TLSRedirect:               tlsRedirectConfig,
			ErrorPageLocations:        errorPageLocations,
			TLSPassthrough:            vsc.isTLSPassthrough,
			Allow:                     policiesCfg.Allow,
			Deny:                      policiesCfg.Deny,
			LimitReqOptions:           policiesCfg.LimitReqOptions,
			LimitReqs:                 policiesCfg.LimitReqs,
			JWTAuth:                   policiesCfg.JWTAuth,
			BasicAuth:                 policiesCfg.BasicAuth,
			JWTAuthList:               policiesCfg.JWTAuthList,
			JWKSAuthEnabled:           policiesCfg.JWKSAuthEnabled,
			IngressMTLS:               policiesCfg.IngressMTLS,
			EgressMTLS:                policiesCfg.EgressMTLS,
			OIDC:                      vsc.oidcPolCfg.oidc,
			WAF:                       policiesCfg.WAF,
			Dos:                       dosCfg,
			PoliciesErrorReturn:       policiesCfg.ErrorReturn,
			VSNamespace:               vsEx.VirtualServer.Namespace,
			VSName:                    vsEx.VirtualServer.Name,
			DisableIPV6:               vsc.isIPV6Disabled,
		},
		SpiffeCerts:             enabledInternalRoutes,
		SpiffeClientCerts:       vsc.spiffeCerts && !enabledInternalRoutes,
		DynamicSSLReloadEnabled: vsc.DynamicSSLReloadEnabled,
		StaticSSLPath:           vsc.StaticSSLPath,
		KeyValZones:             keyValZones,
		KeyVals:                 keyVals,
		TwoWaySplitClients:      twoWaySplitClients,
	}

	return vsCfg, vsc.warnings
}

type policiesCfg struct {
	Allow           []string
	Deny            []string
	LimitReqOptions version2.LimitReqOptions
	LimitReqZones   []version2.LimitReqZone
	LimitReqs       []version2.LimitReq
	JWTAuth         *version2.JWTAuth
	JWTAuthList     map[string]*version2.JWTAuth
	JWKSAuthEnabled bool
	BasicAuth       *version2.BasicAuth
	IngressMTLS     *version2.IngressMTLS
	EgressMTLS      *version2.EgressMTLS
	OIDC            bool
	WAF             *version2.WAF
	ErrorReturn     *version2.Return
	BundleValidator bundleValidator
}

type bundleValidator interface {
	// validate returns the full path to the bundle and an error if the file is not accessible
	validate(string) (string, error)
}

type internalBundleValidator struct {
	bundlePath string
}

func (i internalBundleValidator) validate(bundle string) (string, error) {
	bundle = path.Join(i.bundlePath, bundle)
	_, err := os.Stat(bundle)
	return bundle, err
}

func newInternalBundleValidator(b string) internalBundleValidator {
	return internalBundleValidator{
		bundlePath: b,
	}
}

func newPoliciesConfig(bv bundleValidator) *policiesCfg {
	return &policiesCfg{
		BundleValidator: bv,
	}
}

type policyOwnerDetails struct {
	owner          runtime.Object
	ownerNamespace string
	vsNamespace    string
	vsName         string
}

type policyOptions struct {
	tls         bool
	secretRefs  map[string]*secrets.SecretReference
	apResources *appProtectResourcesForVS
}

type validationResults struct {
	isError  bool
	warnings []string
}

func newValidationResults() *validationResults {
	return &validationResults{}
}

func (v *validationResults) addWarningf(msgFmt string, args ...interface{}) {
	v.warnings = append(v.warnings, fmt.Sprintf(msgFmt, args...))
}

func (p *policiesCfg) addAccessControlConfig(accessControl *conf_v1.AccessControl) *validationResults {
	res := newValidationResults()
	p.Allow = append(p.Allow, accessControl.Allow...)
	p.Deny = append(p.Deny, accessControl.Deny...)
	if len(p.Allow) > 0 && len(p.Deny) > 0 {
		res.addWarningf(
			"AccessControl policy (or policies) with deny rules is overridden by policy (or policies) with allow rules",
		)
	}
	return res
}

func (p *policiesCfg) addRateLimitConfig(
	rateLimit *conf_v1.RateLimit,
	polKey string,
	polNamespace string,
	polName string,
	vsNamespace string,
	vsName string,
) *validationResults {
	res := newValidationResults()
	rlZoneName := fmt.Sprintf("pol_rl_%v_%v_%v_%v", polNamespace, polName, vsNamespace, vsName)
	p.LimitReqs = append(p.LimitReqs, generateLimitReq(rlZoneName, rateLimit))
	p.LimitReqZones = append(p.LimitReqZones, generateLimitReqZone(rlZoneName, rateLimit))
	if len(p.LimitReqs) == 1 {
		p.LimitReqOptions = generateLimitReqOptions(rateLimit)
	} else {
		curOptions := generateLimitReqOptions(rateLimit)
		if curOptions.DryRun != p.LimitReqOptions.DryRun {
			res.addWarningf("RateLimit policy %s with limit request option dryRun='%v' is overridden to dryRun='%v' by the first policy reference in this context", polKey, curOptions.DryRun, p.LimitReqOptions.DryRun)
		}
		if curOptions.LogLevel != p.LimitReqOptions.LogLevel {
			res.addWarningf("RateLimit policy %s with limit request option logLevel='%v' is overridden to logLevel='%v' by the first policy reference in this context", polKey, curOptions.LogLevel, p.LimitReqOptions.LogLevel)
		}
		if curOptions.RejectCode != p.LimitReqOptions.RejectCode {
			res.addWarningf("RateLimit policy %s with limit request option rejectCode='%v' is overridden to rejectCode='%v' by the first policy reference in this context", polKey, curOptions.RejectCode, p.LimitReqOptions.RejectCode)
		}
	}
	return res
}

func (p *policiesCfg) addBasicAuthConfig(
	basicAuth *conf_v1.BasicAuth,
	polKey string,
	polNamespace string,
	secretRefs map[string]*secrets.SecretReference,
) *validationResults {
	res := newValidationResults()
	if p.BasicAuth != nil {
		res.addWarningf("Multiple basic auth policies in the same context is not valid. Basic auth policy %s will be ignored", polKey)
		return res
	}

	basicSecretKey := fmt.Sprintf("%v/%v", polNamespace, basicAuth.Secret)
	secretRef := secretRefs[basicSecretKey]
	var secretType api_v1.SecretType
	if secretRef.Secret != nil {
		secretType = secretRef.Secret.Type
	}
	if secretType != "" && secretType != secrets.SecretTypeHtpasswd {
		res.addWarningf("Basic Auth policy %s references a secret %s of a wrong type '%s', must be '%s'", polKey, basicSecretKey, secretType, secrets.SecretTypeHtpasswd)
		res.isError = true
		return res
	} else if secretRef.Error != nil {
		res.addWarningf("Basic Auth policy %s references an invalid secret %s: %v", polKey, basicSecretKey, secretRef.Error)
		res.isError = true
		return res
	}

	p.BasicAuth = &version2.BasicAuth{
		Secret: secretRef.Path,
		Realm:  basicAuth.Realm,
	}
	return res
}

func (p *policiesCfg) addJWTAuthConfig(
	jwtAuth *conf_v1.JWTAuth,
	polKey string,
	polNamespace string,
	secretRefs map[string]*secrets.SecretReference,
) *validationResults {
	res := newValidationResults()
	if p.JWTAuth != nil {
		res.addWarningf("Multiple jwt policies in the same context is not valid. JWT policy %s will be ignored", polKey)
		return res
	}
	if jwtAuth.Secret != "" {
		jwtSecretKey := fmt.Sprintf("%v/%v", polNamespace, jwtAuth.Secret)
		secretRef := secretRefs[jwtSecretKey]
		var secretType api_v1.SecretType
		if secretRef.Secret != nil {
			secretType = secretRef.Secret.Type
		}
		if secretType != "" && secretType != secrets.SecretTypeJWK {
			res.addWarningf("JWT policy %s references a secret %s of a wrong type '%s', must be '%s'", polKey, jwtSecretKey, secretType, secrets.SecretTypeJWK)
			res.isError = true
			return res
		} else if secretRef.Error != nil {
			res.addWarningf("JWT policy %s references an invalid secret %s: %v", polKey, jwtSecretKey, secretRef.Error)
			res.isError = true
			return res
		}

		p.JWTAuth = &version2.JWTAuth{
			Secret: secretRef.Path,
			Realm:  jwtAuth.Realm,
			Token:  jwtAuth.Token,
		}
		return res
	} else if jwtAuth.JwksURI != "" {
		uri, _ := url.Parse(jwtAuth.JwksURI)

		JwksURI := &version2.JwksURI{
			JwksScheme: uri.Scheme,
			JwksHost:   uri.Hostname(),
			JwksPort:   uri.Port(),
			JwksPath:   uri.Path,
		}

		p.JWTAuth = &version2.JWTAuth{
			Key:      polKey,
			JwksURI:  *JwksURI,
			Realm:    jwtAuth.Realm,
			Token:    jwtAuth.Token,
			KeyCache: jwtAuth.KeyCache,
		}
		p.JWKSAuthEnabled = true
		return res
	}
	return res
}

func (p *policiesCfg) addIngressMTLSConfig(
	ingressMTLS *conf_v1.IngressMTLS,
	polKey string,
	polNamespace string,
	context string,
	tls bool,
	secretRefs map[string]*secrets.SecretReference,
) *validationResults {
	res := newValidationResults()
	if !tls {
		res.addWarningf("TLS must be enabled in VirtualServer for IngressMTLS policy %s", polKey)
		res.isError = true
		return res
	}
	if context != specContext {
		res.addWarningf("IngressMTLS policy %s is not allowed in the %v context", polKey, context)
		res.isError = true
		return res
	}
	if p.IngressMTLS != nil {
		res.addWarningf("Multiple ingressMTLS policies are not allowed. IngressMTLS policy %s will be ignored", polKey)
		return res
	}

	secretKey := fmt.Sprintf("%v/%v", polNamespace, ingressMTLS.ClientCertSecret)
	secretRef := secretRefs[secretKey]
	var secretType api_v1.SecretType
	if secretRef.Secret != nil {
		secretType = secretRef.Secret.Type
	}
	if secretType != "" && secretType != secrets.SecretTypeCA {
		res.addWarningf("IngressMTLS policy %s references a secret %s of a wrong type '%s', must be '%s'", polKey, secretKey, secretType, secrets.SecretTypeCA)
		res.isError = true
		return res
	} else if secretRef.Error != nil {
		res.addWarningf("IngressMTLS policy %q references an invalid secret %s: %v", polKey, secretKey, secretRef.Error)
		res.isError = true
		return res
	}

	verifyDepth := 1
	verifyClient := "on"
	if ingressMTLS.VerifyDepth != nil {
		verifyDepth = *ingressMTLS.VerifyDepth
	}
	if ingressMTLS.VerifyClient != "" {
		verifyClient = ingressMTLS.VerifyClient
	}

	caFields := strings.Fields(secretRef.Path)

	if _, hasCrlKey := secretRef.Secret.Data[CACrlKey]; hasCrlKey && ingressMTLS.CrlFileName != "" {
		res.addWarningf("Both ca.crl in the Secret and ingressMTLS.crlFileName fields cannot be used. ca.crl in %s will be ignored and %s will be applied", secretKey, polKey)
	}

	if ingressMTLS.CrlFileName != "" {
		p.IngressMTLS = &version2.IngressMTLS{
			ClientCert:   caFields[0],
			ClientCrl:    fmt.Sprintf("%s/%s", DefaultSecretPath, ingressMTLS.CrlFileName),
			VerifyClient: verifyClient,
			VerifyDepth:  verifyDepth,
		}
	} else if _, hasCrlKey := secretRef.Secret.Data[CACrlKey]; hasCrlKey {
		p.IngressMTLS = &version2.IngressMTLS{
			ClientCert:   caFields[0],
			ClientCrl:    caFields[1],
			VerifyClient: verifyClient,
			VerifyDepth:  verifyDepth,
		}
	} else {
		p.IngressMTLS = &version2.IngressMTLS{
			ClientCert:   caFields[0],
			VerifyClient: verifyClient,
			VerifyDepth:  verifyDepth,
		}
	}
	return res
}

func (p *policiesCfg) addEgressMTLSConfig(
	egressMTLS *conf_v1.EgressMTLS,
	polKey string,
	polNamespace string,
	secretRefs map[string]*secrets.SecretReference,
) *validationResults {
	res := newValidationResults()
	if p.EgressMTLS != nil {
		res.addWarningf(
			"Multiple egressMTLS policies in the same context is not valid. EgressMTLS policy %s will be ignored",
			polKey,
		)
		return res
	}

	var tlsSecretPath string

	if egressMTLS.TLSSecret != "" {
		egressTLSSecret := fmt.Sprintf("%v/%v", polNamespace, egressMTLS.TLSSecret)

		secretRef := secretRefs[egressTLSSecret]
		var secretType api_v1.SecretType
		if secretRef.Secret != nil {
			secretType = secretRef.Secret.Type
		}
		if secretType != "" && secretType != api_v1.SecretTypeTLS {
			res.addWarningf("EgressMTLS policy %s references a secret %s of a wrong type '%s', must be '%s'", polKey, egressTLSSecret, secretType, api_v1.SecretTypeTLS)
			res.isError = true
			return res
		} else if secretRef.Error != nil {
			res.addWarningf("EgressMTLS policy %s references an invalid secret %s: %v", polKey, egressTLSSecret, secretRef.Error)
			res.isError = true
			return res
		}

		tlsSecretPath = secretRef.Path
	}

	var trustedSecretPath string

	if egressMTLS.TrustedCertSecret != "" {
		trustedCertSecret := fmt.Sprintf("%v/%v", polNamespace, egressMTLS.TrustedCertSecret)

		secretRef := secretRefs[trustedCertSecret]
		var secretType api_v1.SecretType
		if secretRef.Secret != nil {
			secretType = secretRef.Secret.Type
		}
		if secretType != "" && secretType != secrets.SecretTypeCA {
			res.addWarningf("EgressMTLS policy %s references a secret %s of a wrong type '%s', must be '%s'", polKey, trustedCertSecret, secretType, secrets.SecretTypeCA)
			res.isError = true
			return res
		} else if secretRef.Error != nil {
			res.addWarningf("EgressMTLS policy %s references an invalid secret %s: %v", polKey, trustedCertSecret, secretRef.Error)
			res.isError = true
			return res
		}

		trustedSecretPath = secretRef.Path
	}

	if len(trustedSecretPath) != 0 {
		caFields := strings.Fields(trustedSecretPath)
		trustedSecretPath = caFields[0]
	}

	p.EgressMTLS = &version2.EgressMTLS{
		Certificate:    tlsSecretPath,
		CertificateKey: tlsSecretPath,
		Ciphers:        generateString(egressMTLS.Ciphers, "DEFAULT"),
		Protocols:      generateString(egressMTLS.Protocols, "TLSv1 TLSv1.1 TLSv1.2"),
		VerifyServer:   egressMTLS.VerifyServer,
		VerifyDepth:    generateIntFromPointer(egressMTLS.VerifyDepth, 1),
		SessionReuse:   generateBool(egressMTLS.SessionReuse, true),
		ServerName:     egressMTLS.ServerName,
		TrustedCert:    trustedSecretPath,
		SSLName:        generateString(egressMTLS.SSLName, "$proxy_host"),
	}
	return res
}

func (p *policiesCfg) addOIDCConfig(
	oidc *conf_v1.OIDC,
	polKey string,
	polNamespace string,
	secretRefs map[string]*secrets.SecretReference,
	oidcPolCfg *oidcPolicyCfg,
) *validationResults {
	res := newValidationResults()
	if p.OIDC {
		res.addWarningf(
			"Multiple oidc policies in the same context is not valid. OIDC policy %s will be ignored",
			polKey,
		)
		return res
	}

	if oidcPolCfg.oidc != nil {
		if oidcPolCfg.key != polKey {
			res.addWarningf(
				"Only one oidc policy is allowed in a VirtualServer and its VirtualServerRoutes. Can't use %s. Use %s",
				polKey,
				oidcPolCfg.key,
			)
			res.isError = true
			return res
		}
	} else {
		secretKey := fmt.Sprintf("%v/%v", polNamespace, oidc.ClientSecret)
		secretRef := secretRefs[secretKey]

		var secretType api_v1.SecretType
		if secretRef.Secret != nil {
			secretType = secretRef.Secret.Type
		}
		if secretType != "" && secretType != secrets.SecretTypeOIDC {
			res.addWarningf("OIDC policy %s references a secret %s of a wrong type '%s', must be '%s'", polKey, secretKey, secretType, secrets.SecretTypeOIDC)
			res.isError = true
			return res
		} else if secretRef.Error != nil {
			res.addWarningf("OIDC policy %s references an invalid secret %s: %v", polKey, secretKey, secretRef.Error)
			res.isError = true
			return res
		}

		clientSecret := secretRef.Secret.Data[ClientSecretKey]

		redirectURI := oidc.RedirectURI
		if redirectURI == "" {
			redirectURI = "/_codexch"
		}
		scope := oidc.Scope
		if scope == "" {
			scope = "openid"
		}
		authExtraArgs := ""
		if oidc.AuthExtraArgs != nil {
			authExtraArgs = strings.Join(oidc.AuthExtraArgs, "&")
		}

		oidcPolCfg.oidc = &version2.OIDC{
			AuthEndpoint:      oidc.AuthEndpoint,
			AuthExtraArgs:     authExtraArgs,
			TokenEndpoint:     oidc.TokenEndpoint,
			JwksURI:           oidc.JWKSURI,
			ClientID:          oidc.ClientID,
			ClientSecret:      string(clientSecret),
			Scope:             scope,
			RedirectURI:       redirectURI,
			ZoneSyncLeeway:    generateIntFromPointer(oidc.ZoneSyncLeeway, 200),
			AccessTokenEnable: oidc.AccessTokenEnable,
		}
		oidcPolCfg.key = polKey
	}

	p.OIDC = true

	return res
}

func (p *policiesCfg) addWAFConfig(
	waf *conf_v1.WAF,
	polKey string,
	polNamespace string,
	apResources *appProtectResourcesForVS,
) *validationResults {
	res := newValidationResults()
	if p.WAF != nil {
		res.addWarningf("Multiple WAF policies in the same context is not valid. WAF policy %s will be ignored", polKey)
		return res
	}

	if waf.Enable {
		p.WAF = &version2.WAF{Enable: "on"}
	} else {
		p.WAF = &version2.WAF{Enable: "off"}
	}

	if waf.ApPolicy != "" {
		apPolKey := waf.ApPolicy
		hasNamespace := strings.Contains(apPolKey, "/")
		if !hasNamespace {
			apPolKey = fmt.Sprintf("%v/%v", polNamespace, apPolKey)
		}

		if apPolPath, exists := apResources.Policies[apPolKey]; exists {
			p.WAF.ApPolicy = apPolPath
		} else {
			res.addWarningf("WAF policy %s references an invalid or non-existing App Protect policy %s", polKey, apPolKey)
			res.isError = true
			return res
		}
	}

	if waf.ApBundle != "" {
		p.WAF.ApBundle = appProtectBundleFolder + waf.ApBundle
		bundlePath, err := p.BundleValidator.validate(waf.ApBundle)
		if err != nil {
			res.addWarningf("WAF policy %s references an invalid or non-existing App Protect bundle %s", polKey, bundlePath)
			res.isError = true
		}
		p.WAF.ApBundle = bundlePath
	}

	if waf.SecurityLog != nil && waf.SecurityLogs == nil {
		glog.V(2).Info("the field securityLog is deprecated and will be removed in future releases. Use field securityLogs instead")
		waf.SecurityLogs = append(waf.SecurityLogs, waf.SecurityLog)
	}

	if waf.SecurityLogs != nil {
		p.WAF.ApSecurityLogEnable = true
		p.WAF.ApLogConf = []string{}
		for _, loco := range waf.SecurityLogs {
			logDest := generateString(loco.LogDest, defaultLogOutput)

			if loco.ApLogConf != "" {
				logConfKey := loco.ApLogConf
				if !strings.Contains(logConfKey, "/") {
					logConfKey = fmt.Sprintf("%v/%v", polNamespace, logConfKey)
				}
				if logConfPath, ok := apResources.LogConfs[logConfKey]; ok {
					p.WAF.ApLogConf = append(p.WAF.ApLogConf, fmt.Sprintf("%s %s", logConfPath, logDest))
				} else {
					res.addWarningf("WAF policy %s references an invalid or non-existing log config %s", polKey, logConfKey)
					res.isError = true
				}
			}

			if loco.ApLogBundle != "" {
				logBundle, err := p.BundleValidator.validate(loco.ApLogBundle)
				if err != nil {
					res.addWarningf("WAF policy %s references an invalid or non-existing log config bundle %s", polKey, logBundle)
					res.isError = true
				} else {
					p.WAF.ApLogConf = append(p.WAF.ApLogConf, fmt.Sprintf("%s %s", logBundle, logDest))
				}
			}
		}
	}
	return res
}

func (vsc *virtualServerConfigurator) generatePolicies(
	ownerDetails policyOwnerDetails,
	policyRefs []conf_v1.PolicyReference,
	policies map[string]*conf_v1.Policy,
	context string,
	policyOpts policyOptions,
) policiesCfg {
	config := newPoliciesConfig(vsc.bundleValidator)

	for _, p := range policyRefs {
		polNamespace := p.Namespace
		if polNamespace == "" {
			polNamespace = ownerDetails.ownerNamespace
		}

		key := fmt.Sprintf("%s/%s", polNamespace, p.Name)

		if pol, exists := policies[key]; exists {
			var res *validationResults
			switch {
			case pol.Spec.AccessControl != nil:
				res = config.addAccessControlConfig(pol.Spec.AccessControl)
			case pol.Spec.RateLimit != nil:
				res = config.addRateLimitConfig(
					pol.Spec.RateLimit,
					key,
					polNamespace,
					p.Name,
					ownerDetails.vsNamespace,
					ownerDetails.vsName,
				)
			case pol.Spec.JWTAuth != nil:
				res = config.addJWTAuthConfig(pol.Spec.JWTAuth, key, polNamespace, policyOpts.secretRefs)
			case pol.Spec.BasicAuth != nil:
				res = config.addBasicAuthConfig(pol.Spec.BasicAuth, key, polNamespace, policyOpts.secretRefs)
			case pol.Spec.IngressMTLS != nil:
				res = config.addIngressMTLSConfig(
					pol.Spec.IngressMTLS,
					key,
					polNamespace,
					context,
					policyOpts.tls,
					policyOpts.secretRefs,
				)
			case pol.Spec.EgressMTLS != nil:
				res = config.addEgressMTLSConfig(pol.Spec.EgressMTLS, key, polNamespace, policyOpts.secretRefs)
			case pol.Spec.OIDC != nil:
				res = config.addOIDCConfig(pol.Spec.OIDC, key, polNamespace, policyOpts.secretRefs, vsc.oidcPolCfg)
			case pol.Spec.WAF != nil:
				res = config.addWAFConfig(pol.Spec.WAF, key, polNamespace, policyOpts.apResources)
			default:
				res = newValidationResults()
			}
			vsc.addWarnings(ownerDetails.owner, res.warnings)
			if res.isError {
				return policiesCfg{
					ErrorReturn: &version2.Return{Code: 500},
				}
			}
		} else {
			vsc.addWarningf(ownerDetails.owner, "Policy %s is missing or invalid", key)
			return policiesCfg{
				ErrorReturn: &version2.Return{Code: 500},
			}
		}
	}

	return *config
}

func generateLimitReq(zoneName string, rateLimitPol *conf_v1.RateLimit) version2.LimitReq {
	var limitReq version2.LimitReq

	limitReq.ZoneName = zoneName

	if rateLimitPol.Burst != nil {
		limitReq.Burst = *rateLimitPol.Burst
	}
	if rateLimitPol.Delay != nil {
		limitReq.Delay = *rateLimitPol.Delay
	}

	limitReq.NoDelay = generateBool(rateLimitPol.NoDelay, false)
	if limitReq.NoDelay {
		limitReq.Delay = 0
	}

	return limitReq
}

func generateLimitReqZone(zoneName string, rateLimitPol *conf_v1.RateLimit) version2.LimitReqZone {
	return version2.LimitReqZone{
		ZoneName: zoneName,
		Key:      rateLimitPol.Key,
		ZoneSize: rateLimitPol.ZoneSize,
		Rate:     rateLimitPol.Rate,
	}
}

func generateLimitReqOptions(rateLimitPol *conf_v1.RateLimit) version2.LimitReqOptions {
	return version2.LimitReqOptions{
		DryRun:     generateBool(rateLimitPol.DryRun, false),
		LogLevel:   generateString(rateLimitPol.LogLevel, "error"),
		RejectCode: generateIntFromPointer(rateLimitPol.RejectCode, 503),
	}
}

func removeDuplicateLimitReqZones(rlz []version2.LimitReqZone) []version2.LimitReqZone {
	encountered := make(map[string]bool)
	result := []version2.LimitReqZone{}

	for _, v := range rlz {
		if !encountered[v.ZoneName] {
			encountered[v.ZoneName] = true
			result = append(result, v)
		}
	}

	return result
}

func addPoliciesCfgToLocation(cfg policiesCfg, location *version2.Location) {
	location.Allow = cfg.Allow
	location.Deny = cfg.Deny
	location.LimitReqOptions = cfg.LimitReqOptions
	location.LimitReqs = cfg.LimitReqs
	location.JWTAuth = cfg.JWTAuth
	location.BasicAuth = cfg.BasicAuth
	location.EgressMTLS = cfg.EgressMTLS
	location.OIDC = cfg.OIDC
	location.WAF = cfg.WAF
	location.PoliciesErrorReturn = cfg.ErrorReturn
}

func addPoliciesCfgToLocations(cfg policiesCfg, locations []version2.Location) {
	for i := range locations {
		addPoliciesCfgToLocation(cfg, &locations[i])
	}
}

func addDosConfigToLocations(dosCfg *version2.Dos, locations []version2.Location) {
	for i := range locations {
		locations[i].Dos = dosCfg
	}
}

func getUpstreamResourceLabels(owner runtime.Object) version2.UpstreamLabels {
	var resourceType, resourceName, resourceNamespace string

	switch owner := owner.(type) {
	case *conf_v1.VirtualServer:
		resourceType = "virtualserver"
		resourceName = owner.Name
		resourceNamespace = owner.Namespace
	case *conf_v1.VirtualServerRoute:
		resourceType = "virtualserverroute"
		resourceName = owner.Name
		resourceNamespace = owner.Namespace
	}

	return version2.UpstreamLabels{
		ResourceType:      resourceType,
		ResourceName:      resourceName,
		ResourceNamespace: resourceNamespace,
	}
}

func (vsc *virtualServerConfigurator) generateUpstream(
	owner runtime.Object,
	upstreamName string,
	upstream conf_v1.Upstream,
	isExternalNameSvc bool,
	endpoints []string,
	backupEndpoints []string,
) version2.Upstream {
	var upsServers []version2.UpstreamServer
	for _, e := range endpoints {
		s := version2.UpstreamServer{
			Address: e,
		}
		upsServers = append(upsServers, s)
	}
	sort.Slice(upsServers, func(i, j int) bool {
		return upsServers[i].Address < upsServers[j].Address
	})

	var upsBackupServers []version2.UpstreamServer
	for _, be := range backupEndpoints {
		s := version2.UpstreamServer{
			Address: be,
		}
		upsBackupServers = append(upsBackupServers, s)
	}
	sort.Slice(upsBackupServers, func(i, j int) bool {
		return upsBackupServers[i].Address < upsBackupServers[j].Address
	})

	lbMethod := generateLBMethod(upstream.LBMethod, vsc.cfgParams.LBMethod)

	upstreamLabels := getUpstreamResourceLabels(owner)
	upstreamLabels.Service = upstream.Service

	ups := version2.Upstream{
		Name:             upstreamName,
		UpstreamLabels:   upstreamLabels,
		Servers:          upsServers,
		Resolve:          isExternalNameSvc,
		LBMethod:         lbMethod,
		Keepalive:        generateIntFromPointer(upstream.Keepalive, vsc.cfgParams.Keepalive),
		MaxFails:         generateIntFromPointer(upstream.MaxFails, vsc.cfgParams.MaxFails),
		FailTimeout:      generateTimeWithDefault(upstream.FailTimeout, vsc.cfgParams.FailTimeout),
		MaxConns:         generateIntFromPointer(upstream.MaxConns, vsc.cfgParams.MaxConns),
		UpstreamZoneSize: vsc.cfgParams.UpstreamZoneSize,
		BackupServers:    upsBackupServers,
	}

	if vsc.isPlus {
		ups.SlowStart = vsc.generateSlowStartForPlus(owner, upstream, lbMethod)
		ups.Queue = generateQueueForPlus(upstream.Queue, "60s")
		ups.SessionCookie = generateSessionCookie(upstream.SessionCookie)
		ups.NTLM = upstream.NTLM
	}

	return ups
}

func (vsc *virtualServerConfigurator) generateSlowStartForPlus(
	owner runtime.Object,
	upstream conf_v1.Upstream,
	lbMethod string,
) string {
	if upstream.SlowStart == "" {
		return ""
	}

	_, isIncompatible := incompatibleLBMethodsForSlowStart[lbMethod]
	isHash := strings.HasPrefix(lbMethod, "hash")
	if isIncompatible || isHash {
		msgFmt := "Slow start will be disabled for upstream %v because lb method '%v' is incompatible with slow start"
		vsc.addWarningf(owner, msgFmt, upstream.Name, lbMethod)
		return ""
	}

	return generateTime(upstream.SlowStart)
}

func generateHealthCheck(
	upstream conf_v1.Upstream,
	upstreamName string,
	cfgParams *ConfigParams,
) *version2.HealthCheck {
	if upstream.HealthCheck == nil || !upstream.HealthCheck.Enable {
		return nil
	}

	hc := newHealthCheckWithDefaults(upstream, upstreamName, cfgParams)

	if upstream.HealthCheck.Path != "" {
		hc.URI = upstream.HealthCheck.Path
	}

	if upstream.HealthCheck.Interval != "" {
		hc.Interval = generateTime(upstream.HealthCheck.Interval)
	}

	if upstream.HealthCheck.Jitter != "" {
		hc.Jitter = generateTime(upstream.HealthCheck.Jitter)
	}

	if upstream.HealthCheck.KeepaliveTime != "" {
		hc.KeepaliveTime = generateTime(upstream.HealthCheck.KeepaliveTime)
	}

	if upstream.HealthCheck.Fails > 0 {
		hc.Fails = upstream.HealthCheck.Fails
	}

	if upstream.HealthCheck.Passes > 0 {
		hc.Passes = upstream.HealthCheck.Passes
	}

	if upstream.HealthCheck.ConnectTimeout != "" {
		hc.ProxyConnectTimeout = generateTime(upstream.HealthCheck.ConnectTimeout)
	}

	if upstream.HealthCheck.ReadTimeout != "" {
		hc.ProxyReadTimeout = generateTime(upstream.HealthCheck.ReadTimeout)
	}

	if upstream.HealthCheck.SendTimeout != "" {
		hc.ProxySendTimeout = generateTime(upstream.HealthCheck.SendTimeout)
	}

	for _, h := range upstream.HealthCheck.Headers {
		hc.Headers[h.Name] = h.Value
	}

	if upstream.HealthCheck.TLS != nil {
		hc.ProxyPass = fmt.Sprintf("%v://%v", generateProxyPassProtocol(upstream.HealthCheck.TLS.Enable), upstreamName)
	}

	if upstream.HealthCheck.StatusMatch != "" {
		hc.Match = generateStatusMatchName(upstreamName)
	}

	hc.Port = upstream.HealthCheck.Port

	hc.Mandatory = upstream.HealthCheck.Mandatory

	hc.Persistent = upstream.HealthCheck.Persistent

	hc.GRPCStatus = upstream.HealthCheck.GRPCStatus

	hc.GRPCService = upstream.HealthCheck.GRPCService

	return hc
}

func generateSessionCookie(sc *conf_v1.SessionCookie) *version2.SessionCookie {
	if sc == nil || !sc.Enable {
		return nil
	}

	return &version2.SessionCookie{
		Enable:   true,
		Name:     sc.Name,
		Path:     sc.Path,
		Expires:  sc.Expires,
		Domain:   sc.Domain,
		HTTPOnly: sc.HTTPOnly,
		Secure:   sc.Secure,
		SameSite: sc.SameSite,
	}
}

func generateStatusMatchName(upstreamName string) string {
	return fmt.Sprintf("%s_match", upstreamName)
}

func generateUpstreamStatusMatch(upstreamName string, status string) version2.StatusMatch {
	return version2.StatusMatch{
		Name: generateStatusMatchName(upstreamName),
		Code: status,
	}
}

// GenerateExternalNameSvcKey returns the key to identify an ExternalName service.
func GenerateExternalNameSvcKey(namespace string, service string) string {
	return fmt.Sprintf("%v/%v", namespace, service)
}

func generateLBMethod(method string, defaultMethod string) string {
	if method == "" {
		return defaultMethod
	} else if method == "round_robin" {
		return ""
	}
	return method
}

func generateIntFromPointer(n *int, defaultN int) int {
	if n == nil {
		return defaultN
	}
	return *n
}

func upstreamHasKeepalive(upstream conf_v1.Upstream, cfgParams *ConfigParams) bool {
	if upstream.Keepalive != nil {
		return *upstream.Keepalive != 0
	}
	return cfgParams.Keepalive != 0
}

func generateRewrites(path string, proxy *conf_v1.ActionProxy, internal bool, originalPath string, grpcEnabled bool) []string {
	if proxy == nil || proxy.RewritePath == "" {
		if grpcEnabled && internal {
			return []string{"^ $request_uri break"}
		}
		return nil
	}

	if originalPath != "" {
		path = originalPath
	}

	isRegex := false
	if strings.HasPrefix(path, "~") {
		isRegex = true
	}

	trimmedPath := strings.TrimPrefix(strings.TrimPrefix(path, "~"), "*")
	trimmedPath = strings.TrimSpace(trimmedPath)

	var rewrites []string

	if internal {
		// For internal locations only, recover the original request_uri without (!) the arguments.
		// This is necessary, because if we just use $request_uri (which includes the arguments),
		// the rewrite that follows will result in an URI with duplicated arguments:
		// for example, /test%3Fhello=world?hello=world instead of /test?hello=world
		rewrites = append(rewrites, "^ $request_uri_no_args")
	}

	if isRegex {
		rewrites = append(rewrites, fmt.Sprintf(`"^%v" "%v" break`, trimmedPath, proxy.RewritePath))
	} else if internal {
		rewrites = append(rewrites, fmt.Sprintf(`"^%v(.*)$" "%v$1" break`, trimmedPath, proxy.RewritePath))
	}

	return rewrites
}

func generateProxyPassRewrite(path string, proxy *conf_v1.ActionProxy, internal bool) string {
	if proxy == nil || internal {
		return ""
	}

	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "=") {
		return proxy.RewritePath
	}

	return ""
}

func generateProxyPass(tlsEnabled bool, upstreamName string, internal bool, proxy *conf_v1.ActionProxy) string {
	proxyPass := fmt.Sprintf("%v://%v", generateProxyPassProtocol(tlsEnabled), upstreamName)

	if internal && (proxy == nil || proxy.RewritePath == "") {
		return fmt.Sprintf("%v$request_uri", proxyPass)
	}

	return proxyPass
}

func generateProxyPassProtocol(enableTLS bool) string {
	if enableTLS {
		return "https"
	}
	return "http"
}

func generateGRPCPass(grpcEnabled bool, tlsEnabled bool, upstreamName string) string {
	grpcPass := fmt.Sprintf("%v://%v", generateGRPCPassProtocol(tlsEnabled), upstreamName)

	if !grpcEnabled {
		return ""
	}

	return grpcPass
}

func generateGRPCPassProtocol(enableTLS bool) string {
	if enableTLS {
		return "grpcs"
	}
	return "grpc"
}

func generateString(s string, defaultS string) string {
	if s == "" {
		return defaultS
	}
	return s
}

func generateTime(value string) string {
	// it is expected that the value has been validated prior to call generateTime
	parsed, _ := ParseTime(value)
	return parsed
}

func generateTimeWithDefault(value string, defaultValue string) string {
	if value == "" {
		// we don't transform the default value yet
		// this is done for backward compatibility, as the time values in the ConfigMap are not validated yet
		return defaultValue
	}

	return generateTime(value)
}

func generateSnippets(enableSnippets bool, snippet string, defaultSnippets []string) []string {
	if !enableSnippets || snippet == "" {
		return defaultSnippets
	}
	return strings.Split(snippet, "\n")
}

func generateBuffers(s *conf_v1.UpstreamBuffers, defaultS string) string {
	if s == nil {
		return defaultS
	}
	return fmt.Sprintf("%v %v", s.Number, s.Size)
}

func generateBool(s *bool, defaultS bool) bool {
	if s != nil {
		return *s
	}
	return defaultS
}

func generatePath(path string) string {
	// Wrap the regular expression (if present) inside double quotes (") to avoid NGINX parsing errors
	if strings.HasPrefix(path, "~*") {
		return fmt.Sprintf(`~* "%v"`, strings.TrimPrefix(strings.TrimPrefix(path, "~*"), " "))
	}
	if strings.HasPrefix(path, "~") {
		return fmt.Sprintf(`~ "%v"`, strings.TrimPrefix(strings.TrimPrefix(path, "~"), " "))
	}

	return path
}

func generateReturnBlock(text string, code int, defaultCode int) *version2.Return {
	returnBlock := &version2.Return{
		Code: defaultCode,
		Text: text,
	}

	if code != 0 {
		returnBlock.Code = code
	}

	return returnBlock
}

type errorPageDetails struct {
	pages []conf_v1.ErrorPage
	index int
	owner runtime.Object
}

func generateLocation(path string, upstreamName string, upstream conf_v1.Upstream, action *conf_v1.Action,
	cfgParams *ConfigParams, errorPages errorPageDetails, internal bool, proxySSLName string,
	originalPath string, locSnippets string, enableSnippets bool, retLocIndex int, isVSR bool, vsrName string,
	vsrNamespace string, vscWarnings Warnings,
) (version2.Location, *version2.ReturnLocation) {
	locationSnippets := generateSnippets(enableSnippets, locSnippets, cfgParams.LocationSnippets)

	if action.Redirect != nil {
		return generateLocationForRedirect(path, locationSnippets, action.Redirect), nil
	}

	if action.Return != nil {
		return generateLocationForReturn(path, cfgParams.LocationSnippets, action.Return, retLocIndex)
	}

	checkGrpcErrorPageCodes(errorPages, isGRPC(upstream.Type), upstream.Name, vscWarnings)

	return generateLocationForProxying(path, upstreamName, upstream, cfgParams, errorPages.pages, internal,
		errorPages.index, proxySSLName, action.Proxy, originalPath, locationSnippets, isVSR, vsrName, vsrNamespace), nil
}

func generateProxySetHeaders(proxy *conf_v1.ActionProxy) []version2.Header {
	var headers []version2.Header

	hasHostHeader := false

	if proxy != nil && proxy.RequestHeaders != nil {
		for _, h := range proxy.RequestHeaders.Set {
			headers = append(headers, version2.Header{
				Name:  h.Name,
				Value: h.Value,
			})

			if strings.ToLower(h.Name) == "host" {
				hasHostHeader = true
			}
		}
	}

	if !hasHostHeader {
		headers = append(headers, version2.Header{Name: "Host", Value: "$host"})
	}

	return headers
}

func generateProxyPassRequestHeaders(proxy *conf_v1.ActionProxy) bool {
	if proxy == nil || proxy.RequestHeaders == nil {
		return true
	}

	if proxy.RequestHeaders.Pass != nil {
		return *proxy.RequestHeaders.Pass
	}

	return true
}

func generateProxyHideHeaders(proxy *conf_v1.ActionProxy) []string {
	if proxy == nil || proxy.ResponseHeaders == nil {
		return nil
	}

	return proxy.ResponseHeaders.Hide
}

func generateProxyPassHeaders(proxy *conf_v1.ActionProxy) []string {
	if proxy == nil || proxy.ResponseHeaders == nil {
		return nil
	}

	return proxy.ResponseHeaders.Pass
}

func generateProxyIgnoreHeaders(proxy *conf_v1.ActionProxy) string {
	if proxy == nil || proxy.ResponseHeaders == nil {
		return ""
	}

	return strings.Join(proxy.ResponseHeaders.Ignore, " ")
}

func generateProxyAddHeaders(proxy *conf_v1.ActionProxy) []version2.AddHeader {
	if proxy == nil || proxy.ResponseHeaders == nil {
		return nil
	}

	var addHeaders []version2.AddHeader
	for _, h := range proxy.ResponseHeaders.Add {
		addHeaders = append(addHeaders, version2.AddHeader{
			Header: version2.Header{
				Name:  h.Name,
				Value: h.Value,
			},
			Always: h.Always,
		})
	}

	return addHeaders
}

func generateLocationForProxying(path string, upstreamName string, upstream conf_v1.Upstream,
	cfgParams *ConfigParams, errorPages []conf_v1.ErrorPage, internal bool, errPageIndex int,
	proxySSLName string, proxy *conf_v1.ActionProxy, originalPath string, locationSnippets []string, isVSR bool, vsrName string, vsrNamespace string,
) version2.Location {
	return version2.Location{
		Path:                     generatePath(path),
		Internal:                 internal,
		Snippets:                 locationSnippets,
		ProxyConnectTimeout:      generateTimeWithDefault(upstream.ProxyConnectTimeout, cfgParams.ProxyConnectTimeout),
		ProxyReadTimeout:         generateTimeWithDefault(upstream.ProxyReadTimeout, cfgParams.ProxyReadTimeout),
		ProxySendTimeout:         generateTimeWithDefault(upstream.ProxySendTimeout, cfgParams.ProxySendTimeout),
		ClientMaxBodySize:        generateString(upstream.ClientMaxBodySize, cfgParams.ClientMaxBodySize),
		ProxyMaxTempFileSize:     cfgParams.ProxyMaxTempFileSize,
		ProxyBuffering:           generateBool(upstream.ProxyBuffering, cfgParams.ProxyBuffering),
		ProxyBuffers:             generateBuffers(upstream.ProxyBuffers, cfgParams.ProxyBuffers),
		ProxyBufferSize:          generateString(upstream.ProxyBufferSize, cfgParams.ProxyBufferSize),
		ProxyPass:                generateProxyPass(upstream.TLS.Enable, upstreamName, internal, proxy),
		ProxyNextUpstream:        generateString(upstream.ProxyNextUpstream, "error timeout"),
		ProxyNextUpstreamTimeout: generateTimeWithDefault(upstream.ProxyNextUpstreamTimeout, "0s"),
		ProxyNextUpstreamTries:   upstream.ProxyNextUpstreamTries,
		ProxyInterceptErrors:     generateProxyInterceptErrors(errorPages),
		ProxyPassRequestHeaders:  generateProxyPassRequestHeaders(proxy),
		ProxySetHeaders:          generateProxySetHeaders(proxy),
		ProxyHideHeaders:         generateProxyHideHeaders(proxy),
		ProxyPassHeaders:         generateProxyPassHeaders(proxy),
		ProxyIgnoreHeaders:       generateProxyIgnoreHeaders(proxy),
		AddHeaders:               generateProxyAddHeaders(proxy),
		ProxyPassRewrite:         generateProxyPassRewrite(path, proxy, internal),
		Rewrites:                 generateRewrites(path, proxy, internal, originalPath, isGRPC(upstream.Type)),
		HasKeepalive:             upstreamHasKeepalive(upstream, cfgParams),
		ErrorPages:               generateErrorPages(errPageIndex, errorPages),
		ProxySSLName:             proxySSLName,
		ServiceName:              upstream.Service,
		IsVSR:                    isVSR,
		VSRName:                  vsrName,
		VSRNamespace:             vsrNamespace,
		GRPCPass:                 generateGRPCPass(isGRPC(upstream.Type), upstream.TLS.Enable, upstreamName),
	}
}

func generateProxyInterceptErrors(errorPages []conf_v1.ErrorPage) bool {
	return len(errorPages) > 0
}

func generateLocationForRedirect(
	path string,
	locationSnippets []string,
	redirect *conf_v1.ActionRedirect,
) version2.Location {
	code := redirect.Code
	if code == 0 {
		code = 301
	}

	return version2.Location{
		Path:                 path,
		Snippets:             locationSnippets,
		ProxyInterceptErrors: true,
		InternalProxyPass:    fmt.Sprintf("http://%s", nginx418Server),
		ErrorPages: []version2.ErrorPage{
			{
				Name:         redirect.URL,
				Codes:        "418",
				ResponseCode: code,
			},
		},
	}
}

func generateLocationForReturn(path string, locationSnippets []string, actionReturn *conf_v1.ActionReturn,
	retLocIndex int,
) (version2.Location, *version2.ReturnLocation) {
	defaultType := actionReturn.Type
	if defaultType == "" {
		defaultType = "text/plain"
	}
	code := actionReturn.Code
	if code == 0 {
		code = 200
	}

	retLocName := fmt.Sprintf("@return_%d", retLocIndex)

	return version2.Location{
			Path:                 path,
			Snippets:             locationSnippets,
			ProxyInterceptErrors: true,
			InternalProxyPass:    fmt.Sprintf("http://%s", nginx418Server),
			ErrorPages: []version2.ErrorPage{
				{
					Name:         retLocName,
					Codes:        "418",
					ResponseCode: code,
				},
			},
		},
		&version2.ReturnLocation{
			Name:        retLocName,
			DefaultType: defaultType,
			Return: version2.Return{
				Text: actionReturn.Body,
			},
		}
}

type routingCfg struct {
	Maps                     []version2.Map
	SplitClients             []version2.SplitClient
	Locations                []version2.Location
	InternalRedirectLocation version2.InternalRedirectLocation
	ReturnLocations          []version2.ReturnLocation
	KeyValZones              []version2.KeyValZone
	KeyVals                  []version2.KeyVal
	TwoWaySplitClients       []version2.TwoWaySplitClients
}

func generateSplits(
	splits []conf_v1.Split,
	upstreamNamer *upstreamNamer,
	crUpstreams map[string]conf_v1.Upstream,
	VariableNamer *VariableNamer,
	scIndex int,
	cfgParams *ConfigParams,
	errorPages errorPageDetails,
	originalPath string,
	locSnippets string,
	enableSnippets bool,
	retLocIndex int,
	isVSR bool,
	vsrName string,
	vsrNamespace string,
	vscWarnings Warnings,
	WeightChangesDynamicReload bool,
) ([]version2.SplitClient, []version2.Location, []version2.ReturnLocation, []version2.Map, []version2.KeyValZone, []version2.KeyVal, []version2.TwoWaySplitClients) {
	var distributions []version2.Distribution
	var splitClients []version2.SplitClient
	var maps []version2.Map
	var keyValZones []version2.KeyValZone
	var keyVals []version2.KeyVal
	var twoWaySplitClients []version2.TwoWaySplitClients

	for i, s := range splits {
		if s.Weight == 0 {
			continue
		}
		d := version2.Distribution{
			Weight: fmt.Sprintf("%d%%", s.Weight),
			Value:  fmt.Sprintf("/%vsplits_%d_split_%d", internalLocationPrefix, scIndex, i),
		}
		distributions = append(distributions, d)
	}

	if WeightChangesDynamicReload && len(splits) == 2 {
		scs, weightMap := generateSplitsForWeightChangesDynamicReload(splits, scIndex, VariableNamer)
		kvZoneName := VariableNamer.GetNameOfKeyvalZoneForSplitClientIndex(scIndex)
		kvz := version2.KeyValZone{
			Name:  kvZoneName,
			Size:  splitClientsKeyValZoneSize,
			State: fmt.Sprintf("%s/%s.json", keyvalZoneBasePath, kvZoneName),
		}
		kv := version2.KeyVal{
			Key:      VariableNamer.GetNameOfKeyvalKeyForSplitClientIndex(scIndex),
			Variable: VariableNamer.GetNameOfKeyvalForSplitClientIndex(scIndex),
			ZoneName: kvZoneName,
		}
		scWithWeights := version2.TwoWaySplitClients{
			Key:               VariableNamer.GetNameOfKeyvalKeyForSplitClientIndex(scIndex),
			Variable:          VariableNamer.GetNameOfKeyvalForSplitClientIndex(scIndex),
			ZoneName:          kvZoneName,
			Weights:           []int{splits[0].Weight, splits[1].Weight},
			SplitClientsIndex: scIndex,
		}
		splitClients = append(splitClients, scs...)
		maps = append(maps, weightMap)
		keyValZones = append(keyValZones, kvz)
		keyVals = append(keyVals, kv)
		twoWaySplitClients = append(twoWaySplitClients, scWithWeights)
	} else {
		splitClient := version2.SplitClient{
			Source:        "$request_id",
			Variable:      VariableNamer.GetNameForSplitClientVariable(scIndex),
			Distributions: distributions,
		}
		splitClients = append(splitClients, splitClient)
	}

	var locations []version2.Location
	var returnLocations []version2.ReturnLocation

	for i, s := range splits {
		path := fmt.Sprintf("/%vsplits_%d_split_%d", internalLocationPrefix, scIndex, i)
		upstreamName := upstreamNamer.GetNameForUpstreamFromAction(s.Action)
		upstream := crUpstreams[upstreamName]
		proxySSLName := generateProxySSLName(upstream.Service, upstreamNamer.namespace)
		newRetLocIndex := retLocIndex + len(returnLocations)
		loc, returnLoc := generateLocation(path, upstreamName, upstream, s.Action, cfgParams, errorPages, true,
			proxySSLName, originalPath, locSnippets, enableSnippets, newRetLocIndex, isVSR, vsrName, vsrNamespace, vscWarnings)
		locations = append(locations, loc)
		if returnLoc != nil {
			returnLocations = append(returnLocations, *returnLoc)
		}
	}

	return splitClients, locations, returnLocations, maps, keyValZones, keyVals, twoWaySplitClients
}

func generateDefaultSplitsConfig(
	route conf_v1.Route,
	upstreamNamer *upstreamNamer,
	crUpstreams map[string]conf_v1.Upstream,
	VariableNamer *VariableNamer,
	scIndex int,
	cfgParams *ConfigParams,
	errorPages errorPageDetails,
	originalPath string,
	locSnippets string,
	enableSnippets bool,
	retLocIndex int,
	isVSR bool,
	vsrName string,
	vsrNamespace string,
	vscWarnings Warnings,
	weightChangesDynamicReload bool,
) routingCfg {
	scs, locs, returnLocs, maps, keyValZones, keyVals, twoWaySplitClients := generateSplits(route.Splits, upstreamNamer, crUpstreams, VariableNamer, scIndex, cfgParams, errorPages, originalPath, locSnippets, enableSnippets, retLocIndex, isVSR, vsrName, vsrNamespace, vscWarnings, weightChangesDynamicReload)

	var irl version2.InternalRedirectLocation
	if weightChangesDynamicReload && len(route.Splits) == 2 {
		irl = version2.InternalRedirectLocation{
			Path:        route.Path,
			Destination: VariableNamer.GetNameOfMapForSplitClientIndex(scIndex),
		}
	} else {
		irl = version2.InternalRedirectLocation{
			Path:        route.Path,
			Destination: VariableNamer.GetNameForSplitClientVariable(scIndex),
		}
	}

	return routingCfg{
		SplitClients:             scs,
		Locations:                locs,
		InternalRedirectLocation: irl,
		ReturnLocations:          returnLocs,
		Maps:                     maps,
		KeyValZones:              keyValZones,
		KeyVals:                  keyVals,
		TwoWaySplitClients:       twoWaySplitClients,
	}
}

func generateSplitsForWeightChangesDynamicReload(splits []conf_v1.Split, scIndex int, VariableNamer *VariableNamer) ([]version2.SplitClient, version2.Map) {
	var splitClients []version2.SplitClient
	var mapParameters []version2.Parameter
	for i := 0; i <= 100; i++ {
		j := 100 - i
		var split version2.SplitClient
		var distributions []version2.Distribution
		if i > 0 {
			distribution := version2.Distribution{
				Weight: fmt.Sprintf("%d%%", i),
				Value:  fmt.Sprintf("/%vsplits_%d_split_%d", internalLocationPrefix, scIndex, 0),
			}
			distributions = append(distributions, distribution)

		}
		if j > 0 {
			distribution := version2.Distribution{
				Weight: fmt.Sprintf("%d%%", j),
				Value:  fmt.Sprintf("/%vsplits_%d_split_%d", internalLocationPrefix, scIndex, 1),
			}
			distributions = append(distributions, distribution)
		}
		split = version2.SplitClient{
			Source:        "$request_id",
			Variable:      VariableNamer.GetNameOfSplitClientsForWeights(scIndex, i, j),
			Distributions: distributions,
		}
		splitClients = append(splitClients, split)
		mapParameters = append(mapParameters, version2.Parameter{
			Value:  VariableNamer.GetNameOfKeyOfMapForWeights(scIndex, i, j),
			Result: VariableNamer.GetNameOfSplitClientsForWeights(scIndex, i, j),
		})

	}

	var mapDefault version2.Parameter
	var result string
	if splits[0].Weight < splits[1].Weight {
		result = VariableNamer.GetNameOfSplitClientsForWeights(scIndex, 0, 100)
	} else {
		result = VariableNamer.GetNameOfSplitClientsForWeights(scIndex, 100, 0)
	}
	mapDefault = version2.Parameter{Value: "default", Result: result}

	mapParameters = append(mapParameters, mapDefault)

	weightsToSplits := version2.Map{
		Source:     VariableNamer.GetNameOfKeyvalForSplitClientIndex(scIndex),
		Variable:   VariableNamer.GetNameOfMapForSplitClientIndex(scIndex),
		Parameters: mapParameters,
	}

	return splitClients, weightsToSplits
}

func generateMatchesConfig(route conf_v1.Route, upstreamNamer *upstreamNamer, crUpstreams map[string]conf_v1.Upstream,
	VariableNamer *VariableNamer, index int, scIndex int, cfgParams *ConfigParams, errorPages errorPageDetails,
	locSnippets string, enableSnippets bool, retLocIndex int, isVSR bool, vsrName string, vsrNamespace string, vscWarnings Warnings, weightChangesDynamicReload bool,
) routingCfg {
	// Generate maps
	var maps []version2.Map
	var twoWaySplitClients []version2.TwoWaySplitClients

	for i, m := range route.Matches {
		for j, c := range m.Conditions {
			source := getNameForSourceForMatchesRouteMapFromCondition(c)
			variable := VariableNamer.GetNameForVariableForMatchesRouteMap(index, i, j)
			successfulResult := "1"
			if j < len(m.Conditions)-1 {
				successfulResult = VariableNamer.GetNameForVariableForMatchesRouteMap(index, i, j+1)
			}

			params := generateParametersForMatchesRouteMap(c.Value, successfulResult)

			matchMap := version2.Map{
				Source:     source,
				Variable:   variable,
				Parameters: params,
			}
			maps = append(maps, matchMap)
		}
	}

	scLocalIndex := 0

	// Generate the main map
	source := ""
	var params []version2.Parameter
	for i, m := range route.Matches {
		source += VariableNamer.GetNameForVariableForMatchesRouteMap(index, i, 0)

		v := fmt.Sprintf("~^%s1", strings.Repeat("0", i))
		r := fmt.Sprintf("/%vmatches_%d_match_%d", internalLocationPrefix, index, i)
		if len(m.Splits) > 0 {
			if weightChangesDynamicReload && len(m.Splits) == 2 {
				r = VariableNamer.GetNameOfMapForSplitClientIndex(scIndex + scLocalIndex)
				scLocalIndex += splitClientAmountWhenWeightChangesDynamicReload
			} else {
				r = VariableNamer.GetNameForSplitClientVariable(scIndex + scLocalIndex)
				scLocalIndex++
			}
		}

		p := version2.Parameter{
			Value:  v,
			Result: r,
		}
		params = append(params, p)
	}

	defaultResult := fmt.Sprintf("/%vmatches_%d_default", internalLocationPrefix, index)
	if len(route.Splits) > 0 {
		if weightChangesDynamicReload && len(route.Splits) == 2 {
			defaultResult = VariableNamer.GetNameOfMapForSplitClientIndex(scIndex + scLocalIndex)
		} else {
			defaultResult = VariableNamer.GetNameForSplitClientVariable(scIndex + scLocalIndex)
		}
	}

	defaultParam := version2.Parameter{
		Value:  "default",
		Result: defaultResult,
	}
	params = append(params, defaultParam)

	variable := VariableNamer.GetNameForVariableForMatchesRouteMainMap(index)

	mainMap := version2.Map{
		Source:     source,
		Variable:   variable,
		Parameters: params,
	}
	maps = append(maps, mainMap)

	// Generate locations for each match and split client
	var locations []version2.Location
	var returnLocations []version2.ReturnLocation
	var splitClients []version2.SplitClient
	var keyValZones []version2.KeyValZone
	var keyVals []version2.KeyVal
	scLocalIndex = 0

	for i, m := range route.Matches {
		if len(m.Splits) > 0 {
			newRetLocIndex := retLocIndex + len(returnLocations)
			scs, locs, returnLocs, mps, kvzs, kvs, twscs := generateSplits(
				m.Splits,
				upstreamNamer,
				crUpstreams,
				VariableNamer,
				scIndex+scLocalIndex,
				cfgParams,
				errorPages,
				route.Path,
				locSnippets,
				enableSnippets,
				newRetLocIndex,
				isVSR,
				vsrName,
				vsrNamespace,
				vscWarnings,
				weightChangesDynamicReload,
			)
			scLocalIndex += len(scs)
			splitClients = append(splitClients, scs...)
			locations = append(locations, locs...)
			returnLocations = append(returnLocations, returnLocs...)
			maps = append(maps, mps...)
			keyValZones = append(keyValZones, kvzs...)
			keyVals = append(keyVals, kvs...)
			twoWaySplitClients = append(twoWaySplitClients, twscs...)
		} else {
			path := fmt.Sprintf("/%vmatches_%d_match_%d", internalLocationPrefix, index, i)
			upstreamName := upstreamNamer.GetNameForUpstreamFromAction(m.Action)
			upstream := crUpstreams[upstreamName]
			proxySSLName := generateProxySSLName(upstream.Service, upstreamNamer.namespace)
			newRetLocIndex := retLocIndex + len(returnLocations)
			loc, returnLoc := generateLocation(path, upstreamName, upstream, m.Action, cfgParams, errorPages, true,
				proxySSLName, route.Path, locSnippets, enableSnippets, newRetLocIndex, isVSR, vsrName, vsrNamespace, vscWarnings)
			locations = append(locations, loc)
			if returnLoc != nil {
				returnLocations = append(returnLocations, *returnLoc)
			}
		}
	}

	// Generate default splits or default action
	if len(route.Splits) > 0 {
		newRetLocIndex := retLocIndex + len(returnLocations)
		scs, locs, returnLocs, mps, kvzs, kvs, twscs := generateSplits(
			route.Splits,
			upstreamNamer,
			crUpstreams,
			VariableNamer,
			scIndex+scLocalIndex,
			cfgParams,
			errorPages,
			route.Path,
			locSnippets,
			enableSnippets,
			newRetLocIndex,
			isVSR,
			vsrName,
			vsrNamespace,
			vscWarnings,
			weightChangesDynamicReload,
		)
		splitClients = append(splitClients, scs...)
		locations = append(locations, locs...)
		returnLocations = append(returnLocations, returnLocs...)
		maps = append(maps, mps...)
		keyValZones = append(keyValZones, kvzs...)
		keyVals = append(keyVals, kvs...)
		twoWaySplitClients = append(twoWaySplitClients, twscs...)
	} else {
		path := fmt.Sprintf("/%vmatches_%d_default", internalLocationPrefix, index)
		upstreamName := upstreamNamer.GetNameForUpstreamFromAction(route.Action)
		upstream := crUpstreams[upstreamName]
		proxySSLName := generateProxySSLName(upstream.Service, upstreamNamer.namespace)
		newRetLocIndex := retLocIndex + len(returnLocations)
		loc, returnLoc := generateLocation(path, upstreamName, upstream, route.Action, cfgParams, errorPages, true,
			proxySSLName, route.Path, locSnippets, enableSnippets, newRetLocIndex, isVSR, vsrName, vsrNamespace, vscWarnings)
		locations = append(locations, loc)
		if returnLoc != nil {
			returnLocations = append(returnLocations, *returnLoc)
		}
	}

	// Generate an InternalRedirectLocation to the location defined by the main map variable
	irl := version2.InternalRedirectLocation{
		Path:        route.Path,
		Destination: variable,
	}

	return routingCfg{
		Maps:                     maps,
		Locations:                locations,
		InternalRedirectLocation: irl,
		SplitClients:             splitClients,
		ReturnLocations:          returnLocations,
		KeyValZones:              keyValZones,
		KeyVals:                  keyVals,
		TwoWaySplitClients:       twoWaySplitClients,
	}
}

var specialMapParameters = map[string]bool{
	"default":   true,
	"hostnames": true,
	"include":   true,
	"volatile":  true,
}

func generateValueForMatchesRouteMap(matchedValue string) (value string, isNegative bool) {
	if len(matchedValue) == 0 {
		return `""`, false
	}

	if matchedValue[0] == '!' {
		isNegative = true
		matchedValue = matchedValue[1:]
	}

	if _, exists := specialMapParameters[matchedValue]; exists {
		return `\` + matchedValue, isNegative
	}

	return fmt.Sprintf(`"%s"`, matchedValue), isNegative
}

func generateParametersForMatchesRouteMap(matchedValue string, successfulResult string) []version2.Parameter {
	value, isNegative := generateValueForMatchesRouteMap(matchedValue)

	valueResult := successfulResult
	defaultResult := "0"
	if isNegative {
		valueResult = "0"
		defaultResult = successfulResult
	}

	params := []version2.Parameter{
		{
			Value:  value,
			Result: valueResult,
		},
		{
			Value:  "default",
			Result: defaultResult,
		},
	}

	return params
}

func getNameForSourceForMatchesRouteMapFromCondition(condition conf_v1.Condition) string {
	if condition.Header != "" {
		return fmt.Sprintf("$http_%s", strings.ReplaceAll(condition.Header, "-", "_"))
	}

	if condition.Cookie != "" {
		return fmt.Sprintf("$cookie_%s", condition.Cookie)
	}

	if condition.Argument != "" {
		return fmt.Sprintf("$arg_%s", condition.Argument)
	}

	return condition.Variable
}

func (vsc *virtualServerConfigurator) generateSSLConfig(owner runtime.Object, tls *conf_v1.TLS, namespace string,
	secretRefs map[string]*secrets.SecretReference, cfgParams *ConfigParams,
) *version2.SSL {
	if tls == nil {
		return nil
	}

	if tls.Secret == "" {
		if vsc.isWildcardEnabled {
			ssl := version2.SSL{
				HTTP2:           cfgParams.HTTP2,
				Certificate:     pemFileNameForWildcardTLSSecret,
				CertificateKey:  pemFileNameForWildcardTLSSecret,
				RejectHandshake: false,
			}
			return &ssl
		}
		return nil
	}

	secretRef := secretRefs[fmt.Sprintf("%s/%s", namespace, tls.Secret)]
	var secretType api_v1.SecretType
	if secretRef.Secret != nil {
		secretType = secretRef.Secret.Type
	}
	var name string
	var rejectHandshake bool
	if secretType != "" && secretType != api_v1.SecretTypeTLS {
		rejectHandshake = true
		vsc.addWarningf(owner, "TLS secret %s is of a wrong type '%s', must be '%s'", tls.Secret, secretType, api_v1.SecretTypeTLS)
	} else if secretRef.Error != nil {
		rejectHandshake = true
		vsc.addWarningf(owner, "TLS secret %s is invalid: %v", tls.Secret, secretRef.Error)
	} else {
		name = secretRef.Path
	}

	ssl := version2.SSL{
		HTTP2:           cfgParams.HTTP2,
		Certificate:     name,
		CertificateKey:  name,
		RejectHandshake: rejectHandshake,
	}

	return &ssl
}

func generateTLSRedirectConfig(tls *conf_v1.TLS) *version2.TLSRedirect {
	if tls == nil || tls.Redirect == nil || !tls.Redirect.Enable {
		return nil
	}

	redirect := &version2.TLSRedirect{
		Code:    generateIntFromPointer(tls.Redirect.Code, 301),
		BasedOn: generateTLSRedirectBasedOn(tls.Redirect.BasedOn),
	}

	return redirect
}

func generateTLSRedirectBasedOn(basedOn string) string {
	if basedOn == "x-forwarded-proto" {
		return "$http_x_forwarded_proto"
	}
	return "$scheme"
}

func createEndpointsFromUpstream(upstream version2.Upstream) []string {
	var endpoints []string

	for _, server := range upstream.Servers {
		endpoints = append(endpoints, server.Address)
	}

	return endpoints
}

func createUpstreamsForPlus(
	virtualServerEx *VirtualServerEx,
	baseCfgParams *ConfigParams,
	staticParams *StaticConfigParams,
) []version2.Upstream {
	var upstreams []version2.Upstream

	isPlus := true
	upstreamNamer := NewUpstreamNamerForVirtualServer(virtualServerEx.VirtualServer)
	vsc := newVirtualServerConfigurator(baseCfgParams, isPlus, false, staticParams, false, nil)

	for _, u := range virtualServerEx.VirtualServer.Spec.Upstreams {
		isExternalNameSvc := virtualServerEx.ExternalNameSvcs[GenerateExternalNameSvcKey(virtualServerEx.VirtualServer.Namespace, u.Service)]
		if isExternalNameSvc {
			glog.V(3).Infof("Service %s is Type ExternalName, skipping NGINX Plus endpoints update via API", u.Service)
			continue
		}

		upstreamName := upstreamNamer.GetNameForUpstream(u.Name)
		upstreamNamespace := virtualServerEx.VirtualServer.Namespace

		endpointsKey := GenerateEndpointsKey(upstreamNamespace, u.Service, u.Subselector, u.Port)
		endpoints := virtualServerEx.Endpoints[endpointsKey]

		backupEndpoints := []string{}
		if u.Backup != "" {
			backupEndpointsKey := GenerateEndpointsKey(upstreamNamespace, u.Backup, u.Subselector, *u.BackupPort)
			backupEndpoints = virtualServerEx.Endpoints[backupEndpointsKey]
		}
		ups := vsc.generateUpstream(virtualServerEx.VirtualServer, upstreamName, u, isExternalNameSvc, endpoints, backupEndpoints)
		upstreams = append(upstreams, ups)
	}

	for _, vsr := range virtualServerEx.VirtualServerRoutes {
		upstreamNamer = NewUpstreamNamerForVirtualServerRoute(virtualServerEx.VirtualServer, vsr)
		for _, u := range vsr.Spec.Upstreams {
			isExternalNameSvc := virtualServerEx.ExternalNameSvcs[GenerateExternalNameSvcKey(vsr.Namespace, u.Service)]
			if isExternalNameSvc {
				glog.V(3).Infof("Service %s is Type ExternalName, skipping NGINX Plus endpoints update via API", u.Service)
				continue
			}

			upstreamName := upstreamNamer.GetNameForUpstream(u.Name)
			upstreamNamespace := vsr.Namespace

			endpointsKey := GenerateEndpointsKey(upstreamNamespace, u.Service, u.Subselector, u.Port)
			endpoints := virtualServerEx.Endpoints[endpointsKey]

			// BackupService
			backupEndpoints := []string{}
			if u.Backup != "" {
				backupEndpointsKey := GenerateEndpointsKey(upstreamNamespace, u.Backup, u.Subselector, *u.BackupPort)
				backupEndpoints = virtualServerEx.Endpoints[backupEndpointsKey]
			}
			ups := vsc.generateUpstream(vsr, upstreamName, u, isExternalNameSvc, endpoints, backupEndpoints)
			upstreams = append(upstreams, ups)
		}
	}

	return upstreams
}

func createUpstreamServersConfigForPlus(upstream version2.Upstream) nginx.ServerConfig {
	if len(upstream.Servers) == 0 {
		return nginx.ServerConfig{}
	}
	return nginx.ServerConfig{
		MaxFails:    upstream.MaxFails,
		FailTimeout: upstream.FailTimeout,
		MaxConns:    upstream.MaxConns,
		SlowStart:   upstream.SlowStart,
	}
}

func generateQueueForPlus(upstreamQueue *conf_v1.UpstreamQueue, defaultTimeout string) *version2.Queue {
	if upstreamQueue == nil {
		return nil
	}

	return &version2.Queue{
		Size:    upstreamQueue.Size,
		Timeout: generateTimeWithDefault(upstreamQueue.Timeout, defaultTimeout),
	}
}

func generateErrorPageName(errPageIndex int, index int) string {
	return fmt.Sprintf("@error_page_%v_%v", errPageIndex, index)
}

func checkGrpcErrorPageCodes(errorPages errorPageDetails, isGRPC bool, uName string, vscWarnings Warnings) {
	if errorPages.pages == nil || !isGRPC {
		return
	}

	var c []int
	for _, e := range errorPages.pages {
		for _, code := range e.Codes {
			if grpcConflictingErrors[code] {
				c = append(c, code)
			}
		}
	}
	if len(c) > 0 {
		vscWarnings.AddWarningf(errorPages.owner, "The error page configuration for the upstream %s is ignored for status code(s) %v, which cannot be used for GRPC upstreams.", uName, c)
	}
}

func generateErrorPageCodes(codes []int) string {
	var c []string
	for _, code := range codes {
		c = append(c, strconv.Itoa(code))
	}
	return strings.Join(c, " ")
}

func generateErrorPages(errPageIndex int, errorPages []conf_v1.ErrorPage) []version2.ErrorPage {
	var ePages []version2.ErrorPage

	for i, e := range errorPages {
		var code int
		var name string

		if e.Redirect != nil {
			code = 301
			if e.Redirect.Code != 0 {
				code = e.Redirect.Code
			}
			name = e.Redirect.URL
		} else {
			code = e.Return.Code
			name = generateErrorPageName(errPageIndex, i)
		}

		ep := version2.ErrorPage{
			Name:         name,
			Codes:        generateErrorPageCodes(e.Codes),
			ResponseCode: code,
		}

		ePages = append(ePages, ep)
	}

	return ePages
}

func generateErrorPageLocations(errPageIndex int, errorPages []conf_v1.ErrorPage) []version2.ErrorPageLocation {
	var errorPageLocations []version2.ErrorPageLocation
	for i, e := range errorPages {
		if e.Redirect != nil {
			// Redirects are handled in the error_page of the location directly, no need for a named location.
			continue
		}

		var headers []version2.Header

		for _, h := range e.Return.Headers {
			headers = append(headers, version2.Header{
				Name:  h.Name,
				Value: h.Value,
			})
		}

		defaultType := "text/html"
		if e.Return.Type != "" {
			defaultType = e.Return.Type
		}

		epl := version2.ErrorPageLocation{
			Name:        generateErrorPageName(errPageIndex, i),
			DefaultType: defaultType,
			Return:      generateReturnBlock(e.Return.Body, 0, 0),
			Headers:     headers,
		}

		errorPageLocations = append(errorPageLocations, epl)
	}

	return errorPageLocations
}

func generateProxySSLName(svcName, ns string) string {
	return fmt.Sprintf("%s.%s.svc", svcName, ns)
}

// isTLSEnabled checks whether TLS is enabled for the given upstream, taking into account the configuration
// of the NGINX Service Mesh and the presence of SPIFFE certificates.
func isTLSEnabled(upstream conf_v1.Upstream, hasSpiffeCerts, isInternalRoute bool) bool {
	if isInternalRoute {
		// Internal routes in the NGINX Service Mesh do not require TLS.
		return false
	}

	// TLS is enabled if explicitly configured for the upstream or if SPIFFE certificates are present.
	return upstream.TLS.Enable || hasSpiffeCerts
}

func isGRPC(protocolType string) bool {
	return protocolType == "grpc"
}

func generateDosCfg(dosResource *appProtectDosResource) *version2.Dos {
	if dosResource == nil {
		return nil
	}
	dos := &version2.Dos{}
	dos.Enable = dosResource.AppProtectDosEnable
	dos.Name = dosResource.AppProtectDosName
	dos.ApDosMonitorURI = dosResource.AppProtectDosMonitorURI
	dos.ApDosMonitorProtocol = dosResource.AppProtectDosMonitorProtocol
	dos.ApDosMonitorTimeout = dosResource.AppProtectDosMonitorTimeout
	dos.ApDosAccessLogDest = dosResource.AppProtectDosAccessLogDst
	dos.ApDosPolicy = dosResource.AppProtectDosPolicyFile
	dos.ApDosSecurityLogEnable = dosResource.AppProtectDosLogEnable
	dos.ApDosLogConf = dosResource.AppProtectDosLogConfFile
	return dos
}
