# kokoa-proxy - Project Plan Issues and Clarifications (Updated: 2025/11/21)

This document summarizes potential issues, ambiguities, and areas for clarification previously identified in `spec.md` and `architecture.md` for the kokoa-proxy project. All previously identified issues have now been addressed or clarified by recent updates to the project documentation.

## Summary of Improvements and Resolutions

The project documentation has been significantly updated, particularly in `spec.md` and `architecture.md`, to address the concerns raised. Key improvements include:

*   **Comprehensive Clarification of Failover Automation and "Bulletproof Hosting"**:
    *   The documentation now clearly distinguishes between automated DNS failover (within 30 seconds) and automatic VPS replacement, which is presented as an optional, configurable feature.
    *   A dedicated section "DDoS対策のスコープと限界" (Scope and Limitations of DDoS Countermeasures) has been added, providing clear context on Nginx's capabilities (Layer 7) versus limitations (Layer 3/4 volumetric attacks), and highlighting the disposable VPS strategy as the core defense. Temporary use of Cloudflare Proxy for large-scale attacks is also discussed. This has fully addressed the initial ambiguity.

*   **Detailed Health Check Strategy**:
    *   The "Health Checker Module" section in `architecture.md` has been vastly expanded. It now includes detailed specifications for health check targets (Edge Nodes and Origins), examples of active and passive checks, clear anomaly detection criteria with defined actions, and a more comprehensive `HealthCheckConfig` API interface. This provides excellent clarity on how health checks are performed and managed.

*   **Enhanced Secure Management of API Tokens and Credentials**:
    *   Explicit security warnings and recommended practices for managing API tokens and credentials in production environments (e.g., Kubernetes Secrets, environment variables with encryption) have been added to `spec.md` and exemplified in `architecture.md`. This reinforces the importance of secure credential handling.

*   **Clearer DDoS Mitigation Limitations**:
    *   The newly added "DDoS対策のスコープと限界" sections in both `spec.md` and `architecture.md` provide a thorough discussion on the capabilities and limitations of Nginx-based DDoS protection, clarifying that it is primarily for application-level attacks and external services may be required for volumetric attacks.

*   **Addressed `client_max_body_size` Security Implications**:
    *   `spec.md` now includes a clear security warning regarding the `client_max_body_size` setting, recommending adjustment based on application requirements to prevent resource exhaustion attacks.

*   **Resolved Cloudflare Origin Certificate Inconsistency**:
    *   `spec.md` explicitly clarifies that the Cloudflare Origin Certificate option is **not recommended** for this project's goal of "Cloudflare Proxy OFF (DNS only)," resolving the previous contradiction and providing guidance for its appropriate use if Cloudflare Proxy were to be enabled.

*   **Clarified `proxy_protocol` Usage**:
    *   `spec.md` now includes a detailed explanation and a stronger recommendation for enabling `proxy_protocol` when accurate client IP logging and backend security features are required, clearly outlining its importance and implications.

*   **Comprehensive End-to-End Automation for VPS Lifecycle**:
    *   `architecture.md` now features an extensive "VPS自動交換の完全なエンドツーエンドフロー" section, presented as a detailed sequence diagram covering all phases from attack detection to new VPS provisioning, configuration, and integration. This provides exceptional clarity on the full automation workflow.

*   **Detailed Control Plane High Availability (HA) Strategies**:
    *   `architecture.md` includes a new "Control Plane高可用性（HA）戦略" section, offering multiple well-defined HA options (Kubernetes, Docker Compose with shared DB, Active-Passive) tailored for different deployment scales, thereby fully addressing the need for Control Plane redundancy.

*   **Addressed Control Plane Module Scalability**:
    *   `architecture.md` now features a dedicated "モジュールのスケーラビリティ特性" (Module Scalability Characteristics) section. This section provides a detailed breakdown for each Control Plane module, classifying their statefulness, scalability, recommended replicas, and specific scaling strategies, thereby comprehensively resolving the concern regarding module-specific scaling challenges.

All previously identified potential problems, ambiguities, and areas for clarification have been thoroughly addressed and clarified in the updated project documentation. The project plan is now significantly more robust and detailed.