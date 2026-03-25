# K8s 部署指南（自托管）

本文档介绍如何将 memorix-server 容器化并部署到自托管 Kubernetes 集群（kubeadm / k3s / k3d 等）。

> **还没部署过 memorix？** 先看 [TiDB Serverless 快速上手](quickstart-tidb.md) 了解基本配置，再回来做 K8s 部署。

---

## 目录

1. [目录结构](#1-目录结构)
2. [构建镜像](#2-构建镜像)
3. [创建 Namespace 和 Secret](#3-创建-namespace-和-secret)
4. [Deployment](#4-deployment)
5. [Service](#5-service)
6. [Ingress（Nginx）](#6-ingressnginx)
7. [完整部署流程](#7-完整部署流程)
8. [滚动更新](#8-滚动更新)
9. [健康检查说明](#9-健康检查说明)
10. [生产建议](#10-生产建议)

---

## 1. 目录结构

建议将所有 K8s 配置文件放在项目的 `deploy/k8s/` 目录下：

```
deploy/k8s/
├── namespace.yaml
├── secret.yaml          # 不提交到 Git！加入 .gitignore
├── configmap.yaml
├── deployment.yaml
├── service.yaml
└── ingress.yaml
```

---

## 2. 构建镜像

项目根目录已有 `server/Dockerfile`，多阶段构建，最终镜像基于 `alpine:3.19`，体积约 20MB。

```bash
# 构建并推送到私有 Registry
REGISTRY="your-registry.example.com"
VERSION="v1.0.0"
IMAGE="${REGISTRY}/memorix-server:${VERSION}"

# 构建（在项目根目录执行）
docker build -f server/Dockerfile -t ${IMAGE} .

# 或者用 Makefile
REGISTRY=your-registry.example.com COMMIT=$(git rev-parse --short HEAD) make docker

# 推送
docker push ${IMAGE}
```

如果使用本地 k3s / k3d，可以直接导入镜像无需 Registry：

```bash
# k3s
docker save ${IMAGE} | sudo k3s ctr images import -

# k3d
k3d image import ${IMAGE} -c <cluster-name>
```

---

## 3. 创建 Namespace 和 Secret

### namespace.yaml

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: memorix
```

### 创建 Secret（命令行方式，不写入文件）

敏感配置（DSN、API Key）直接用 `kubectl create secret` 创建，**不要写入 YAML 文件提交到 Git**。

```bash
kubectl -n memorix create secret generic memorix-secrets \
  --from-literal=MNEMO_DSN="user:pass@tcp(tidb-host:4000)/memorix?parseTime=true&tls=true" \
  --from-literal=MNEMO_LLM_API_KEY="sk-..." \
  --from-literal=MNEMO_LLM_BASE_URL="https://api.openai.com/v1"
```

如果使用 TiDB Auto-Embed（不需要外部 Embedding 服务）：

```bash
kubectl -n memorix create secret generic memorix-secrets \
  --from-literal=MNEMO_DSN="user:pass@tcp(tidb-host:4000)/memorix?parseTime=true&tls=true" \
  --from-literal=MNEMO_LLM_API_KEY="sk-..." \
  --from-literal=MNEMO_LLM_BASE_URL="https://api.openai.com/v1"
```

验证 Secret 创建成功：

```bash
kubectl -n memorix get secret memorix-secrets
```

### configmap.yaml

非敏感配置用 ConfigMap，可以安全提交到 Git：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: memorix-config
  namespace: memorix
data:
  MNEMO_PORT: "8080"
  MNEMO_LLM_MODEL: "gpt-4o-mini"
  MNEMO_LLM_TEMPERATURE: "0.1"
  MNEMO_INGEST_MODE: "smart"
  MNEMO_EMBED_AUTO_MODEL: "tidbcloud_free/amazon/titan-embed-text-v2"
  MNEMO_EMBED_AUTO_DIMS: "1024"
  MNEMO_FTS_ENABLED: "false"
  MNEMO_GC_ENABLED: "true"
  MNEMO_GC_INTERVAL: "24h"
  MNEMO_GC_MAX_MEMORIES_PER_TENANT: "10000"
  MNEMO_MAX_CONTEXT_TOKENS: "8192"
  MNEMO_RATE_LIMIT: "100"
  MNEMO_RATE_BURST: "200"
  MNEMO_TIDB_ZERO_ENABLED: "true"
  MNEMO_UPLOAD_DIR: "/data/uploads"
  MNEMO_WORKER_CONCURRENCY: "5"
  TZ: "Asia/Shanghai"
```

---

## 4. Deployment

### deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: memorix-server
  namespace: memorix
  labels:
    app: memorix-server
spec:
  replicas: 2
  selector:
    matchLabels:
      app: memorix-server
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0        # 保证滚动更新期间始终有实例在线
  template:
    metadata:
      labels:
        app: memorix-server
    spec:
      containers:
        - name: memorix-server
          image: your-registry.example.com/memorix-server:v1.0.0
          imagePullPolicy: Always
          ports:
            - containerPort: 8080
              name: http

          # 从 ConfigMap 注入非敏感配置
          envFrom:
            - configMapRef:
                name: memorix-config

          # 从 Secret 注入敏感配置
          env:
            - name: MNEMO_DSN
              valueFrom:
                secretKeyRef:
                  name: memorix-secrets
                  key: MNEMO_DSN
            - name: MNEMO_LLM_API_KEY
              valueFrom:
                secretKeyRef:
                  name: memorix-secrets
                  key: MNEMO_LLM_API_KEY
            - name: MNEMO_LLM_BASE_URL
              valueFrom:
                secretKeyRef:
                  name: memorix-secrets
                  key: MNEMO_LLM_BASE_URL

          # 健康检查
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3

          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3

          # 资源限制（根据实际负载调整）
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"

          # 文件上传目录持久化
          volumeMounts:
            - name: upload-data
              mountPath: /data/uploads

      volumes:
        - name: upload-data
          persistentVolumeClaim:
            claimName: memorix-uploads-pvc

      # 如果私有 Registry 需要认证
      # imagePullSecrets:
      #   - name: registry-credentials
```

### PVC（文件上传持久化）

如果使用文件上传功能（`/imports` 端点），需要挂载持久卷；否则可以删除 `volumeMounts` 和 `volumes` 部分。

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: memorix-uploads-pvc
  namespace: memorix
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  # storageClassName: standard   # 按集群实际存储类名填写
```

---

## 5. Service

### service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: memorix-server
  namespace: memorix
  labels:
    app: memorix-server
spec:
  selector:
    app: memorix-server
  ports:
    - name: http
      port: 80
      targetPort: 8080
      protocol: TCP
  type: ClusterIP
```

---

## 6. Ingress（Nginx）

需要集群中已安装 [ingress-nginx](https://kubernetes.github.io/ingress-nginx/)。

### ingress.yaml

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: memorix-server
  namespace: memorix
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "120"    # LLM 调用可能较慢
    nginx.ingress.kubernetes.io/proxy-send-timeout: "120"
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"       # 支持文件上传
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
    - host: memorix.your-domain.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: memorix-server
                port:
                  name: http
  # HTTPS（需要 cert-manager 或手动证书）
  # tls:
  #   - hosts:
  #       - memorix.your-domain.com
  #     secretName: memorix-tls
```

### 启用 HTTPS（推荐，使用 cert-manager）

安装 cert-manager 后，在 Ingress annotations 中添加：

```yaml
annotations:
  cert-manager.io/cluster-issuer: letsencrypt-prod
```

并取消 `tls` 部分的注释。

---

## 7. 完整部署流程

```bash
# 1. 应用 namespace
kubectl apply -f deploy/k8s/namespace.yaml

# 2. 创建 Secret（命令行，不写文件）
kubectl -n memorix create secret generic memorix-secrets \
  --from-literal=MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true&tls=true" \
  --from-literal=MNEMO_LLM_API_KEY="sk-..." \
  --from-literal=MNEMO_LLM_BASE_URL="https://api.openai.com/v1"

# 3. 应用其余配置（可以全部合并一条命令）
kubectl apply -f deploy/k8s/configmap.yaml
kubectl apply -f deploy/k8s/deployment.yaml
kubectl apply -f deploy/k8s/service.yaml
kubectl apply -f deploy/k8s/ingress.yaml

# 或者一次性应用整个目录（Secret 文件除外）
kubectl apply -f deploy/k8s/ --exclude=secret.yaml

# 4. 确认 Pod 正常运行
kubectl -n memorix get pods
kubectl -n memorix get svc
kubectl -n memorix get ingress

# 5. 查看启动日志
kubectl -n memorix logs -l app=memorix-server --tail=50

# 6. 验证健康检查
kubectl -n memorix exec deploy/memorix-server -- wget -qO- http://localhost:8080/healthz
```

---

## 8. 滚动更新

```bash
# 更新镜像版本（触发滚动更新）
kubectl -n memorix set image deployment/memorix-server \
  memorix-server=your-registry.example.com/memorix-server:v1.1.0

# 查看滚动更新进度
kubectl -n memorix rollout status deployment/memorix-server

# 回滚到上一个版本（出问题时）
kubectl -n memorix rollout undo deployment/memorix-server

# 查看历史版本
kubectl -n memorix rollout history deployment/memorix-server
```

---

## 9. 健康检查说明

memorix-server 在 `/healthz` 提供健康检查端点，返回 `{"status":"ok"}`。

| Probe | 路径 | 作用 |
|---|---|---|
| **livenessProbe** | `/healthz` | 容器无响应时自动重启 Pod |
| **readinessProbe** | `/healthz` | Pod 未就绪时不接收流量，滚动更新期间保证零宕机 |

`initialDelaySeconds: 5` 为服务留出数据库连接初始化时间。如果数据库在远程（如 TiDB Cloud），可适当延长到 15s。

---

## 10. 生产建议

### 副本数与 HPA

单实例即可处理中等负载，生产环境建议 2 副本以上保证高可用。如需弹性伸缩：

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: memorix-server
  namespace: memorix
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: memorix-server
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

### 多副本注意事项

memorix-server 是**无状态**的，所有状态在 TiDB 数据库中。多副本部署天然支持，无需任何额外配置。

唯一需要注意的是文件上传目录（`/data/uploads`）：多副本共享时需要将 PVC 的 `accessModes` 改为 `ReadWriteMany`（需要 NFS 或云存储 CSI）。如果不使用文件上传功能，可以忽略此项。

### Secret 更新

更新 API Key 等 Secret 后，需要重启 Pod 才能生效（环境变量不会热重载）：

```bash
# 更新 Secret
kubectl -n memorix create secret generic memorix-secrets \
  --from-literal=MNEMO_LLM_API_KEY="sk-new-key..." \
  --dry-run=client -o yaml | kubectl apply -f -

# 重启 Pod 使新 Secret 生效
kubectl -n memorix rollout restart deployment/memorix-server
```

### 日志收集

Pod 日志输出为结构化 JSON（slog），可直接对接 Fluentd / Vector / Loki。日志级别通过 `MNEMO_LOG_LEVEL` 控制（默认 `info`）。

### 资源预估

| 场景 | CPU Request | Memory Request |
|---|---|---|
| 开发 / 轻量 | 50m | 64Mi |
| 生产（默认） | 100m | 128Mi |
| 高并发（>100 req/s） | 500m | 256Mi |

LLM 和 Embedding 调用为外部 HTTP，不占用 memorix-server 本身的 CPU/Memory，主要消耗在网络 I/O 等待上。
