---
title: About
description: "This document describes the F5 NGINX Ingress Controller, an Ingress Controller implementation for NGINX and NGINX Plus."
weight: 100
doctypes: ["concept"]
docs: "DOCS-612"
---

<br>

The NGINX Ingress Controller is an [Ingress Controller]({{< relref "glossary.md#ingress-controller">}}) implementation for NGINX and NGINX Plus that can load balance Websocket, gRPC, TCP and UDP applications. It supports standard [Ingress]({{< relref "glossary.md#ingress">}}) features such as content-based routing and TLS/SSL termination. Several NGINX and NGINX Plus features are available as extensions to Ingress resources through [Annotations]({{< relref "configuration/ingress-resources/advanced-configuration-with-annotations">}}) and the [ConfigMap]({{< relref "configuration/global-configuration/configmap-resource">}}) resource.

The NGINX Ingress Controller supports the [VirtualServer and VirtualServerRoute resources]({{< relref "configuration/virtualserver-and-virtualserverroute-resources">}}) as alternatives to Ingress, enabling traffic splitting and advanced content-based routing. It also supports TCP, UDP and TLS Passthrough load balancing using [TransportServer resources]({{< relref "configuration/transportserver-resource">}}).

To learn more about the NGINX Ingress Controller, please read the [How NGINX Ingress Controller is Designed
]({{< relref "overview/design">}}) and [Extensibility with NGINX Plus]({{< relref "overview/nginx-plus">}}) pages.
