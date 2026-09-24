@'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    uid: prometheus-ds
    access: proxy
    url: http://localhost:9090
    isDefault: true
    editable: true
'@ | Set-Content -Path "C:\Program Files\GrafanaLabs\grafana\conf\provisioning\datasources\push_arch_bin_sync.yaml" -Encoding utf8

@'
apiVersion: 1

providers:
  - name: "push_arch_bin_sync"
    orgId: 1
    folder: ""
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: true
    options:
      path: C:\Users\Justin Louis\Documents\push_arch_bin_sync\monitoring\grafana\dashboards
      foldersFromFilesStructure: false
'@ | Set-Content -Path "C:\Program Files\GrafanaLabs\grafana\conf\provisioning\dashboards\push_arch_bin_sync.yaml" -Encoding utf8

Restart-Service Grafana
Write-Host "Done. Grafana restarted with the new datasource + dashboard."
