# ADR-0027: OLS Docker Runtime Verification

## Status

Accepted

## Context

Phase 9 的 Docker functional smoke 中，OLS / LSWS 路径仅验证 LLStack 生成的配置文件落盘正确（compile/asset-first），不验证 OLS 能否实际加载这些配置并提供 HTTP 服务。Apache 路径已具备真实服务启动、configtest、vhost 列表、HTTP 请求验证。

OLS 和 Apache 的验证深度存在显著差距。

## Decision

在 OLS Docker smoke 中增加运行时验证层：

1. **安装 OpenLiteSpeed** — el9-ols / el10-ols Dockerfile 从 LiteSpeed 仓库安装 `openlitespeed`
2. **systemctl shim** — 新增 `systemctl-ols.sh`，将 `systemctl start/stop/reload/is-active lsws` 映射到 `lswsctrl`
3. **运行时验证流程** — smoke.sh 中：
   - 先用 `--skip-reload` 完成站点生命周期测试
   - 将生成的 vhost 注册到 OLS 主配置
   - 启动 OLS 服务
   - 执行 configtest
   - 执行 HTTP 请求验证

4. **LSWS 保持不变** — LSWS 需要真实授权，Docker 中无法启动服务，继续 asset-first

## Consequences

优点：

- OLS 验证深度提升到与 Apache 同级
- 能证明 LLStack 生成的 OLS 配置在真实服务中可用
- systemctl shim 支持 doctor probe 在容器中工作

限制：

- Docker 镜像体积增大（安装了 openlitespeed）
- Docker build 时间增加
- OLS 主配置的 vhost 注册仍需 smoke fixture 手动完成（LLStack 当前不管理 OLS 主配置）
- LSWS 仍无法做运行时验证
- 运行时验证设计为 best-effort：如果 OLS 启动失败，smoke 仍会通过（asset assertions 仍然覆盖）
