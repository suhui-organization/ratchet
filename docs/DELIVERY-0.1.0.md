# 交付记录 0.1.0

> 日期：2026-09-18。工程位置：`/home/walden/Workspaces/ratchet`。
> 本文是实测记录：命令原文、关键输出、以及**没做完的部分**。

## 1. 这一版是什么

一个能走通的垂直切片：**工具清单 → 最小权限策略 → 可被收货方独立验证的交付包**。

不是演示稿：三端都有测试，且都用真实命令跑过。

## 2. 交付物

| # | 产物 | 位置 | 状态 |
|---|---|---|---|
| 1 | Go 引擎：能力判定 + 策略编译 + CLI | `cmd/ratchet`、`internal/{model,policy}` | ✅ 有测试 |
| 2 | 清单契约与策略模型（跨语言共用） | `internal/model/model.go` | ✅ |
| 3 | Python 交付物层：sha256 清单 | `service/src/ratchet_service/manifest.py` | ✅ 有测试 |
| 4 | 单文件独立验证器（仅标准库） | `service/src/ratchet_service/verify.py` | ✅ 实测可在隔离环境跑 |
| 5 | 合规条款映射（EU AI Act / ISO 42001，6 条） | `service/src/ratchet_service/compliance.py` | ✅ 有测试 |
| 6 | 分享包（一个 JSON，拖进网页即可验） | `service/.../cli.py` 的 `bundle` | ✅ 有测试 |
| 7 | Web：落地页 + 验证页（Nuxt4 + Vue3 + Tailwind） | `web/app/` | ✅ 构建通过 |
| 8 | 浏览器端 WebCrypto 校验逻辑 | `web/app/utils/verify.ts` | ✅ 有测试 |
| 9 | 决策与计划落档 | `docs/DECISIONS.md`、`docs/PLAN.md` | ✅ |

## 3. 测试记录

### Go

```console
$ go vet ./... && go test ./...
ok  github.com/suhui-organization/ratchet/internal/policy
```

用例覆盖：能力判定（26 个具名用例，含 `delete_file` / `deleteFile` / `filesystem.delete_file`
三种写法）、危险度优先级（`read_and_delete_records` 必须判成破坏性）、
描述回退（`"Permanently removes the given record"` → destructive）、
**确定性**（同一输入两次编译必须逐字节相同）、去重、未知工具进待确认、`--strict-unknown`。

### Python

```console
$ cd service && python3 -m pytest -q
14 passed
```

含：清单不自引用、篡改被检出并点名、文件缺失、**空清单不算通过**、
以及"收货方在隔离环境里跑单个文件"（`env -i` + 无 PYTHONPATH）。

### Web

```console
$ cd web && npm run test
✓ tests/verify.test.ts (8 tests)
$ npm run build
✨ Build complete!
```

### 端到端（手工跑过一遍）

```console
$ ratchet policy draft --from examples/inventory.json --out policy.json
  工具      9 → allow 4 · approve 4 · deny 1
  待确认    1（无法判定能力，已置为 approve，请核对后再用）
  DENY     filesystem/delete_file             名称命中「delete」；观测到 1 次调用
  APPROVE  shell/execute_command              名称命中「exec」；观测到 17 次调用
  ALLOW    filesystem/read_file               名称命中「read」；观测到 120 次调用

$ ratchet-report build --dir <交付目录>
  产物 2 份

$ env -i PATH=/usr/bin:/bin HOME=/tmp python3 verify.py <交付目录>
✅ Verification passed: 2 file(s) match manifest.json
```

最后一条是关键：**收货方用一个文件、不装包、不设环境变量**就验完了。

## 4. 开发中发现并修掉的问题（都是测试抓到的）

| 问题 | 现象 | 修法 |
|---|---|---|
| 描述里的英文词形变化判不出来 | `"Permanently removes…"` 判成 unknown（词表里只有 `remove`） | 长词走前缀匹配（`delete` 命中 `deleted`/`deletion`），短词（`sh`）仍精确匹配 |
| 去重后依据没更新 | 依据写着"观测到 1 次"，实际是 5 次 | 改成"先合并、再判定"，依据一定基于最终数据 |
| 空清单会 vacuous 通过 | `{"artifacts": []}` 得到"验证通过" | 空清单判失败并给出原因 |
| 错误文案不跟语言走 | 英文界面里混中文错误 | 错误改成 key + 参数，由渲染层翻译 |
| 同工具名跨 server 误判 | `decisionOf` 只按工具名匹配 | 改为按 `server/tool` 精确匹配 |

## 5. 没做完的（明确列出）

| # | 事项 | 为什么没做 | 下一步 |
|---|---|---|---|
| L1 | `ratchet scan`：真实读 harness 配置、连 MCP `tools/list` | 本版只定义了清单契约，扫描是独立的下一步 | P1.5 |
| L2 | 报告渲染（Markdown，含覆盖边界章节） | 交付目录现在要手工放 `report.md` | P2.4 |
| L3 | 后端 API（按 token 取交付包） | 本版用"一个 JSON 文件"绕开了服务端——更符合本地优先 | P3.3，可选 |
| L4 | 30 天复验 | 依赖 L3 | P3.4 |
| L5 | CLI 中英双语 | 本版只有中文；国际化会显著扩大改动面 | 视海外使用情况 |
| L6 | 交付目录里放验证器的副本 | 会让验证器自己也进清单，牵动 manifest 语义 | P2.5 |

## 6. 环境备注

- **Go 未预装**，本次装到 `~/.local/go`（用户级，未动系统目录）。
  使用前需要 `export PATH="$HOME/.local/go/bin:$PATH"`。
- `web/` 的依赖用 `npm install` 装过（626 个包）。
- 版本库已初始化并完成首次提交。
