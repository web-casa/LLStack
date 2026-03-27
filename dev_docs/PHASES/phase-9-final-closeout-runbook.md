# Phase 9 Final Closeout Runbook

## Purpose

本 runbook 用于在具备 Docker daemon 权限的环境中完成 Phase 9 的最终结项。

当前唯一 release gate：

- 执行一次真实全矩阵 Docker functional smoke
- 记录结果
- 将 Phase 9 从 `in_progress` 改为 `completed`

## Preconditions

执行前必须满足：

- 当前机器可访问 Docker daemon
- `docker info` 可成功执行
- 仓库工作目录位于项目根目录
- 当前代码与文档已是最新状态

建议先验证：

```bash
docker info
go test ./...
go build ./...
docker compose -f docker/compose/functional.yaml config --services
```

预期服务矩阵：

- `el9-apache`
- `el9-ols`
- `el9-lsws`
- `el10-apache`
- `el10-ols`
- `el10-lsws`

## Execution Steps

### 1. Run Real Docker Smoke

优先执行：

```bash
make docker-smoke
```

等价入口：

```bash
LLSTACK_RUN_DOCKER_TESTS=1 go test ./tests/integration/docker -run TestDockerFunctionalSmoke -v
```

### 2. Verify Artifacts

确认 `dist/docker-smoke/` 下生成每个服务的日志：

```bash
ls dist/docker-smoke
```

预期至少包含：

- `el9-apache.log`
- `el9-ols.log`
- `el9-lsws.log`
- `el10-apache.log`
- `el10-ols.log`
- `el10-lsws.log`

每份日志都应包含结构化成功标记：

```text
"status": "passed"
```

生成统一摘要：

```bash
make docker-smoke-report
```

预期生成：

- `dist/docker-smoke/summary.json`

### 3. Spot-Check Assertions

建议额外抽查：

- Apache 日志中存在真实 `httpd` 配置测试、reload 或 HTTP 响应断言
- OLS 日志中存在 `vhconf.conf` / parity report 断言
- LSWS 日志中存在 managed include / parity report 断言

### 4. Record Result

将执行日期、环境和结果写回：

- [TESTING.md](/home/web-casa/llstack/dev_docs/TESTING.md)
- [phase-9.md](/home/web-casa/llstack/dev_docs/PHASES/phase-9.md)
- [ROADMAP.md](/home/web-casa/llstack/dev_docs/ROADMAP.md)
- [closeout-review.md](/home/web-casa/llstack/dev_docs/PHASES/closeout-review.md)

## Success Criteria

满足以下全部条件即可视为 Phase 9 正式结项：

- `docker info` 成功
- `make docker-smoke` 成功退出
- 全部 6 个服务都生成 smoke artifact
- 全部 6 个 artifact 都包含 passed marker
- `dist/docker-smoke/summary.json` 存在且 `overall_status=passed`
- 未发现需要阻塞发布的 matrix 失败项
- 文档已回写真实执行记录

## Required Documentation Updates

### 1. TESTING.md

需要补：

- 执行日期
- 执行机器/环境说明
- 使用的命令
- 真实运行结果
- 如有失败或降级，写明原因

### 2. phase-9.md

需要把：

- `当前状态：Phase 9 in progress`

改为：

- `当前状态：Phase 9 completed`

并删除或更新：

- “当前环境未执行真实容器 smoke” 相关阻塞描述

### 3. ROADMAP.md

需要把：

- `Phase 9 | in_progress`

改为：

- `Phase 9 | completed`

并把 `Current Phase` 改成后续 backlog 或 release maintenance 描述。

### 4. closeout-review.md

需要把：

- `Path B: Conditional Closeout`

更新为：

- `Path A: Full Closeout`

并记录真实 Docker smoke 已完成。

## Failure Handling

如果真实 smoke 失败，不要直接改 `completed`。

应按下面流程处理：

1. 保留 `Phase 9 in_progress`
2. 记录失败服务与失败日志路径
3. 判断是：
   - 环境权限问题
   - Docker image/build 问题
   - backend runtime 问题
   - smoke assertion 问题
4. 仅修复阻塞项，不扩大范围
5. 重新执行真实 smoke

## Post-Closeout Backlog

Phase 9 正式结项后，剩余项进入后续 backlog：

- artifact signing
- trusted provenance chain
- 完整 CI/CD workflow
- 更广泛的 Docker / e2e matrix
- 大规模 UX 重构
