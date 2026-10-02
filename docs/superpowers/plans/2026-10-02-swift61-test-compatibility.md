# Swift 6.1 测试兼容性 Implementation Plan

> **执行状态：已完成。** 按用户的 AGENTS.md 在主线程顺序执行。测试修复提交为 `181d7bd`；原有 777 个测试全部通过。后续网络与刷新行为回归加入后，当前共 787 个测试通过。

**Goal:** 在本机 Xcode 16.4 / Swift 6.1.2 下编译并运行现有测试，为后续二开提供可靠的回归检查。

**Architecture:** 保留现有三个私有 URLProtocol 测试替身及其串行 suite，修正异步响应桥接函数的参数类型。修改仅作用于测试代码；正常应用构建和打包继续使用已验证的工具链。

**Tech Stack:** Swift 6.1.2、SwiftPM、Swift Testing、Foundation URLProtocol。

## Chunk 1：修复测试编译

### 已核实的事实

- 2026-10-02 再次执行 `swift test -v`，退出码 1，仍在三个测试替身的 `Task` 捕获处编译失败。
- 三个替身类已经声明 `@unchecked Sendable`；响应桥接函数却把参数写成 Foundation 的基类 `URLProtocol`，丢失具体类型的并发传递声明。
- 局部变量的 `nonisolated(unsafe)` 无法消除本机编译器报出的跨任务传递问题。
- 临时提取三个测试替身，以 Swift 6 模式、arm64 / macOS 12 deployment target 编译对象文件：三个原始样例均失败；将桥接参数改为对应具体类、删除局部变量上的 `nonisolated(unsafe)` 后，三个候选样例均成功。
- 上述实验只验证了候选类型修正的可编译性，尚未应用候选修复后完成仓库的完整测试，也没有证明 `@unchecked Sendable` 自动具备运行时线程安全。仅执行 `swiftc -typecheck` 不足以复现此错误，必须实际编译。

### Task 1：修正三个响应桥接函数

**Files / Test：**

| 文件 | 桥接函数所在类型 | 参数应保留的类型 |
| --- | --- | --- |
| `Tests/CodexRunwayCoreTests/GrokTokenRefresherTests.swift` | `NonisolatedURLProtocolResponder` | `MockGrokTokenURLProtocol` |
| `Tests/CodexRunwayCoreTests/GrokBillingClientTests.swift` | `NonisolatedBillingURLProtocolResponder` | `MockBillingURLProtocol` |
| `Tests/CodexRunwayCoreTests/GrokAccountModuleTests.swift` | `NonisolatedModuleTokenURLProtocolResponder` | `MockGrokModuleTokenURLProtocol` |

- [x] 在干净的功能分支复现 `swift test` 编译失败，记录工具链版本。
- [x] 对三个桥接函数分别替换参数类型。例如第一个函数：

```swift
nonisolated static func fulfill(
    request: URLRequest,
    handler: @escaping @Sendable (URLRequest) async throws -> (HTTPURLResponse, Data),
    protocol urlProtocol: MockGrokTokenURLProtocol)
```

- [x] 将三个局部声明统一改为普通常量，并更新相关注释：

```swift
let client = urlProtocol.client
let protocolInstance = urlProtocol
Task {
    do {
        let (response, data) = try await handler(request)
        client?.urlProtocol(protocolInstance, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(protocolInstance, didLoad: data)
        client?.urlProtocolDidFinishLoading(protocolInstance)
    } catch {
        client?.urlProtocol(protocolInstance, didFailWithError: error)
    }
}
```

- [x] 检查已有 suite 串行约束、handler 捕获和请求完成顺序，保留现有测试覆盖。若运行中出现取消后的回调或共享状态问题，应单独复现并处理；串行 suite 本身不保证每个异步回调安全。
- [x] 执行定向测试：

```bash
swift test --filter 'Grok(TokenRefresher|BillingClient|AccountModule)Tests'
```

**预期：** 测试目标可编译，三个 suite 执行完成并通过。此命令仍会编译测试目标，因此可以发现剩余编译错误。

### Task 2：完整回归与记录

- [x] 执行 `swift test`，以实际结果记录通过数量和 suite 数量。上游的 777 个测试仅作为历史对照，不能直接记为本机结果。
- [x] 执行 `swift build`，确认主程序构建仍通过。
- [x] 更新 `docs/development/baseline-2026-10-02.md` 和 `docs/development/local-run.md` 的后续进展，保留最初失败记录。
- [x] 独立提交：`test: preserve concrete URLProtocol types on Swift 6.1`。

**验收：** 本机完整 `swift test` 和 `swift build` 退出码均为 0；编译器并发检查保持启用；现有测试没有被删除、跳过或削弱断言。测试修复不改变安装版行为。

### 实际执行中补充的修复

- `GrokAccountModuleTests.refreshPersistsStableErrorCodes` 使用无 refresh token 的凭据夹具，阻止错误码测试意外发起真实 OAuth 刷新；真实 token 刷新继续由原有替身用例覆盖。
- `NetworkProxyPreviewTests` 将滚动夹具视口统一为 280pt，使本机 302–315pt 的实际内容仍必然测试滚动。
- `ControlPanelTabBarTests` 按长标签是否超出紧凑窗口、扩展后是否容纳完整控件来断言，移除仅适用于另一系统的 722pt 阈值。
- 原有测试没有被删除或跳过；保持 Swift 6 并发检查。定向、完整测试和主程序构建均已完成。

### 备选路径（本次无需使用）

若完整项目仍出现候选样例未覆盖的工具链限制，先定位新增失败，再评估与上游 Swift 6.3.3 对齐。升级前核对该工具链所需的 macOS / Xcode 版本，使用独立安装和命令级 `DEVELOPER_DIR` 做验证，保留当前能正常打包的环境。

**推荐：** 先执行小范围测试修复；工具链升级作为备选，不作为当前启动与二开的前置条件。

## 与网络方案的衔接

本计划通过后再实施 [联网超时处理方案](2026-10-02-network-reliability.md)，这样网络重试、取消和账号隔离的回归测试有可用的执行基线。
