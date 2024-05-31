---
title: "Troubleshooting with NGINX App Protect WAF"
description: "This document describes how to troubleshoot problems when using NGINX Ingress Controller and the NGINX App Protect WAF module."
weight: 700
docs: "DOCS-1460"
doctypes: [""]
toc: true
aliases:
 - /content/troubleshooting/troubleshooting-with-app-protect
---

This document describes how to troubleshoot problems with NGINX Ingress Controller with the [App Protect](/nginx-app-protect/) module enabled.

For general troubleshooting of NGINX Ingress Controller, check the general [troubleshooting]({{< relref "troubleshooting/troubleshoot-common" >}}) documentation.

{{< see-also >}} You can find more troubleshooting tips in the NGINX App Protect WAF [troubleshooting guide](/nginx-app-protect/troubleshooting/) {{< /see-also >}}.

## Potential Problems

The table below categorizes some potential problems with the Ingress Controller when App Protect WAF module is enabled. It suggests how to troubleshoot those problems, using one or more methods from the next section.

{{% table %}}
|Problem area | Symptom | Troubleshooting method | Common cause |
| ---| ---| ---| --- |
|Start. | The Ingress Controller fails to start. | Check the logs. | Misconfigured APLogConf or APPolicy. |
|APLogConf, APPolicy or Ingress Resource. | The configuration is not applied. | Check the events of the APLogConf, APPolicy and Ingress Resource, check the logs, replace the policy. | APLogConf or APPolicy is invalid. |
|NGINX. | The Ingress Controller NGINX verification timeouts while starting for the first time or while reloading after a change. | Check the logs for ``Unable to fetch version: X`` message. Check the Availability of APPolicy External References. | Too many Ingress Resources with App Protect enabled. Check the `NGINX fails to start/reload section <#nginx-fails-to-start-or-reload>`_ of the Known Issues. |
{{% /table %}}

## Troubleshooting Methods

### Check NGINX Ingress Controller and App Protect logs

App Protect logs are part of NGINX Ingress Controller logs when the module is enabled. To check NGINX Ingress Controller logs, follow the steps of [Checking the Ingress Controller Logs]({{< relref "troubleshooting/troubleshoot-common#checking-the-ingress-controller-logs" >}}) of the Troubleshooting guide.

For App Protect specific logs, look for messages starting with `APP_PROTECT`, for example:

```
2020/07/10 11:13:20 [notice] 17#17: APP_PROTECT { "event": "configuration_load_success", "software_version": "2.52.1", "completed_successfully":true,"attack_signatures_package":{"revision_datetime":"2020-06-18T10:11:32Z","version":"2020.06.18"}}
```

### Check events of an Ingress Resource

Follow the steps of [Checking the Events of an Ingress Resource]({{< relref "troubleshooting/troubleshoot-ingress" >}}).

### Check events of APLogConf

After you create or update an APLogConf, you can immediately check if the NGINX configuration was successfully applied by NGINX:

```shell
kubectl describe aplogconf logconf
Name:         logconf
Namespace:    default

Events:
  Type    Reason          Age   From                      Message
  ----    ------          ----  ----                      -------
  Normal  AddedOrUpdated  11s   nginx-ingress-controller  AppProtectLogConfig  default/logconf was added or updated
```

Note that in the events section, we have a `Normal` event with the `AddedOrUpdated` reason, which informs us that the configuration was successfully applied.

### Check events of APPolicy

After you create or update an APPolicy, you can immediately check if the NGINX configuration was successfully applied by NGINX:

```shell
kubectl describe appolicy dataguard-alarm
Name:         dataguard-alarm
Namespace:    default

Events:
  Type    Reason          Age    From                      Message
  ----    ------          ----   ----                      -------
  Normal  AddedOrUpdated  2m25s  nginx-ingress-controller  AppProtectPolicy default/dataguard-alarm was added or updated
```

The events section has a *Normal* event with the *AddedOrUpdated reason*, indicating the policy was successfully accepted.

### Replace the Policy

NOTE: This method only applies if using [external references](/nginx-app-protect/configuration/#external-references)
If items on the external reference change but the spec of the APPolicy remains unchanged (even when re-applying the policy), Kubernetes will not detect the update.
In this case you can force-replace the resource. This will remove the resource and add it again, triggering a reload. For example:

```shell
kubectl replace appolicy -f your-policy-manifest.yaml --force
```

### Check the Availability of APPolicy External References

NOTE: This method only applies if you're using [external references](/nginx-app-protect/configuration/#external-references) in NGINX App Protect policies.

To check what servers host the external references of a policy:

```shell
kubectl get appolicy mypolicy -o jsonpath='{.items[*].spec.policy.*.link}' | tr ' ' '\n'
http://192.168.100.100/resources/headersettings.txt
```

You can check the total time a http request takes, in multiple ways eg. using curl:

```shell
curl -w '%{time_total}' http://192.168.100.100/resources/headersettings.txt
```

## Run App Protect in Debug Mode

When you set NGINX Ingress Controller to use debug mode, the setting also applies to the App Protect WAF module.  See  [Running NGINX in the Debug Mode]({{< relref "troubleshooting/troubleshoot-common.md#running-nginx-in-the-debug-mode" >}}) for instructions.

## Known Issues

When using NGINX Ingress Controller with the App Protect WAF module, the following issues have been reported. The occurrence of these issues is commonly related to a higher number of Ingress Resources with App Protect being enabled in a cluster.

When you make a change that requires NGINX to apply a new configuration, the Ingress Controller reloads NGINX automatically. Without the App Protect WAF module enabled, usual reload times are around 150ms. If App Protect WAF module is enabled and is being used by any number of Ingress Resources, these reloads might take a few seconds instead.

### NGINX Configuration Skew

If you are running more than one instance of the Ingress Controller, the extended reload time may cause the NGINX configuration of your instances to be out of sync. This can occur because there is no order imposed on how the Ingress Controller processes the Kubernetes Resources. The configurations will be the same after all instances have completed the reload.

In order to reduce these inconsistencies, we advise that you do not apply changes to multiple resources handled by the Ingress Controller at the same time.

### NGINX Fails to Start or Reload

The first time NGINX Ingress Controller starts, or whenever there is a change that requires reloading NGINX, NGINX Ingress Controller will verify if the reload was successful. The timeout for this verification is normally 4 seconds. When App Protect is enabled, this timeout increases to 20 seconds.

This timeout should be more than enough to verify configurations. However, when numerous Ingress resources with App Protect enabled are handled by NGINX Ingress Controller at the same time, you may find that you need to extend the timeout further. Examples of when this might be necessary include:

- You need to apply a large amount of Ingress Resources at once.
- You are running NGINX Ingress Controller for the first time in a cluster where the Ingress resources with App Protect enabled are already present.

You can increase this timeout by setting the `nginx-reload-timeout` [cli-argument]({{< relref "configuration/global-configuration/command-line-arguments.md#cmdoption-nginx-reload-timeout" >}}).

When using the User Defined Signature feature, an update to an `APUserSig` requires more reload time from NGINX Plus compared with the other AppProtect resources. As a consequence, we recommend increasing the `nginx-reload-timeout` to 30 seconds if you're planning to use this feature.

If you are using external references in your NGINX App Protect policies, verify if the servers hosting the referenced resources are available and that their response time is as short as possible (see the Check the Availability of APPolicy External References section). If the references are not available during NGINX Ingress Controller startup, the pod will fail to start. In case the resources are not available during a reload, the reload will fail, and NGINX Plus will use the previous correct configuration.
