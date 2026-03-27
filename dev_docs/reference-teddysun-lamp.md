# Research: teddysun/lamp

Sources:

- GitHub README: https://github.com/teddysun/lamp
- Script entrypoint: https://raw.githubusercontent.com/teddysun/lamp/master/lamp.sh

## 1. 值得学习

### 安装体验直给

- 支持系统、软件版本、架构、默认路径写得很清楚，用户心智负担低。
- 安装入口单一，命令短，适合小内存 VPS 和新机器首装。
- 默认值明确，交互成本低，适合快速上线。

### 支持矩阵表达清晰

- README 直接列出支持系统、软件版本、架构、默认位置、管理命令。
- 对运维用户非常友好，先回答“能不能用”，再回答“怎么用”。

### 默认路径与运维动作统一

- Web root、配置路径、服务管理命令有一致约定。
- 对低经验用户有明显价值，减少“文件到底在哪里”的摩擦。

### 小机器友好

- 明确 512 MiB RAM 级别的使用场景。
- 安装流程偏 package manager 驱动，降低编译成本与失败率。

## 2. 不建议继承

### Bash 单体脚本耦合过重

- 安装、系统初始化、服务配置、交互、错误处理都堆在一个入口脚本内。
- 难以形成 planner / renderer / verifier / rollback 的清晰边界。

### 以安装为中心，不是生命周期产品

- 强项是快速装好 LAMP，但站点生命周期、回滚、诊断、能力抽象不是核心设计中心。
- 不适合作为多 backend、per-site PHP、多 DB provider 产品的内核架构。

### 配置抽象不足

- 没有 canonical model，也没有“同一站点定义渲染到多个 backend”的抽象。
- 如果直接继承，会破坏 LLStack 的 Apache semantics -> canonical IR -> backend compiler 架构。

### 可测试性有限

- Bash 脚本对单元测试、golden tests、快照回归、fake executor 都不友好。
- 后续 Docker 功能测试也更难切分层次。

## 3. 我们的改进版设计原则

1. 保留它的优点：支持矩阵清晰、默认值明确、上手快、小机器友好。
2. 升级它的架构：用 Go 单二进制、统一 executor、planner/applier/rollbacker、结构化状态。
3. 从“安装脚本”升级为“生命周期管理工具”：安装只是一个入口，不是产品全部。
4. 以 canonical model 为核心，而不是以某个 backend 配置文件为核心实现细节。
5. 明确 multi-backend 抽象，不能因为参考项目偏 Apache/LAMP 就牺牲 OLS / LSWS。
6. 交互层双轨并存：CLI 与 TUI 共用同一 plan/apply 逻辑，不允许两套平行实现。
7. 所有关键输出可测试：renderer golden tests、plan snapshot tests、Docker 功能测试矩阵。
