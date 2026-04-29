# Traefik Component Design Document

**Component**: `platform/networking/ingress/traefik`  
**Date**: 2026-04-29  
**Status**: Implemented  
**Context**: Traefik migration to ArgoCD with ProxyProtocol optimization for BGP architecture

## Problem Statement

During the Traefik migration from Helm to ArgoCD management, the existing ProxyProtocol configuration was identified as unnecessary for the current network architecture. The infrastructure had evolved from HAProxy-based reverse proxy to direct Cilium BGP routing, making ProxyProtocol redundant while adding unnecessary complexity.

## Current Network Architecture

**Traffic Flow**: `Client → Edge Router → Cilium BGP → Traefik LoadBalancer → Backend Services`

**Key Characteristics**:
- Direct BGP routing via Cilium BGP controller
- No intermediate proxy/load balancer layer
- LoadBalancer service IP `172.16.180.1` advertised via BGP
- Client IP addresses naturally preserved (no masking)
- ProxyProtocol not required for client IP preservation

**Previous Architecture** (deprecated):
- HAProxy reverse proxy in front of services
- ProxyProtocol required to preserve client IPs through proxy layer

## Design Requirements

1. **Remove ProxyProtocol**: Disable unnecessary ProxyProtocol for current BGP architecture
2. **Future Flexibility**: Enable easy re-activation if proxy layer added in future
3. **Documentation**: Clear guidance on when and how to enable ProxyProtocol
4. **Maintainability**: Configuration should be self-documenting and obvious to operators

## Solution Design

### Configuration Approach
**Selected**: Commented Configuration with Documentation

Alternative approaches considered:
- Complete removal (rejected - not future-flexible)
- Boolean toggle with conditionals (rejected - overly complex for this use case)
- Environment-based configuration (rejected - not applicable)

### Implementation Structure

```yaml
# Network Architecture Configuration
# Current: Direct BGP routing via Cilium BGP to LoadBalancer services
# Future: If adding proxy/load balancer in front of Traefik, uncomment proxyProtocol

ports:
  web:
    port: 8000
    # ProxyProtocol Configuration
    # Enable when adding proxy/load balancer in front of Traefik
    # proxyProtocol:
    #   trustedIPs:
    #   - 10.0.0.0/8       # Private network ranges
    #   - 172.16.0.0/12    # Docker/container networks  
    #   - 192.168.0.0/16   # Local network ranges
```

### Documentation Components

1. **Inline Documentation**: Network architecture explanation in values.yaml
2. **Component README**: Comprehensive operational documentation
3. **Migration Context**: Clear rationale for configuration decisions
4. **Future Instructions**: Step-by-step re-enablement process

## Implementation Details

### Files Modified
- `values.yaml`: ProxyProtocol sections commented out with documentation
- `README.md`: Complete component documentation created

### Key Documentation Sections
- Network architecture and traffic flow
- When to enable ProxyProtocol (future scenarios)
- Configuration examples and trusted IP explanations
- Operational runbooks and troubleshooting guides

### Migration Integration
- Applied during ongoing Helm-to-ArgoCD migration
- Maintains all existing functionality
- Reduces configuration complexity
- Improves performance (eliminates unnecessary protocol parsing)

## Benefits

1. **Simplified Configuration**: Removes unnecessary complexity for current architecture
2. **Performance**: Eliminates ProxyProtocol parsing overhead
3. **Future-Proof**: Easy to re-enable when needed
4. **Self-Documenting**: Configuration explains itself and architecture context
5. **Operational Clarity**: Clear guidance for when to make changes

## Testing Strategy

1. **Functionality Verification**: Ensure ingress routing works without ProxyProtocol
2. **Client IP Preservation**: Verify real client IPs reach backend applications
3. **Dashboard Access**: Confirm dashboard authentication and access logs
4. **Certificate Management**: Validate TLS certificates and auto-renewal

## Rollback Plan

If issues arise:
1. Uncomment ProxyProtocol sections in values.yaml
2. Adjust trustedIPs if needed
3. Apply configuration via ArgoCD sync
4. Verify client IP preservation works correctly

## Future Considerations

**When to Re-enable ProxyProtocol**:
- Adding HAProxy/nginx/cloud load balancer in front of Traefik
- Moving to multi-tier architecture with proxy layer
- Implementing WAF or DDoS protection with proxy

**Configuration Updates Required**:
- Uncomment proxyProtocol sections
- Update trustedIPs to match proxy network ranges
- Test client IP preservation through proxy chain
- Update documentation to reflect new architecture

## Success Criteria

✅ **Configuration Simplified**: ProxyProtocol disabled for BGP architecture  
✅ **Documentation Complete**: Comprehensive README and inline documentation  
✅ **Future-Flexible**: Easy re-enablement process documented  
✅ **Functionality Preserved**: All ingress and dashboard features working  
✅ **Client IPs Preserved**: Real client IPs reaching applications  

## References

- **Network Evolution**: Git history showing HAProxy → BGP transition
- **Cilium BGP**: Infrastructure configuration for LoadBalancer IP advertisement  
- **ProxyProtocol Spec**: [HAProxy ProxyProtocol documentation](https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt)
- **Traefik Documentation**: [ProxyProtocol configuration](https://doc.traefik.io/traefik/routing/entrypoints/#proxyprotocol)