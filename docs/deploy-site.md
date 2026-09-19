# 部署站点

站点是**纯静态**的：落地页 + 验证页（哈希在浏览器里算，不需要服务端）。
所以任何静态托管都能用，而且不需要任何环境变量或密钥。

```bash
cd web
npm ci
npm run generate      # 产物在 web/.output/public
```

产物长这样：

```
.output/public/
├── index.html          # 落地页
├── verify/index.html   # 验证页
├── fonts/              # 自托管字体（不请求第三方 CDN）
└── _nuxt/              # JS / CSS
```

---

## Vercel（推荐，与 pod-website 同一套）

仓库里已经有 `web/vercel.json`，写死了构建命令与产物目录，所以**只需要在面板里设一项**：

1. Vercel → Add New → Project → 选 `suhui-organization/ratchet`
2. **Root Directory 填 `web`**（关键；不填会找不到项目）
3. 其余保持默认（`vercel.json` 会接管 build 与 output）
4. Deploy

之后每次 push 到 `main` 都会自动重新部署。

绑域名时注意：`podcloud.dlszjr.com` 现在指向旧站点，
要改到 ratchet 需要在 **Vercel → Project → Settings → Domains** 里加这个域名，
然后去 DNS 把记录指到 Vercel 给的 CNAME。

## 自己的服务器 / 对象存储

`web/.output/public` 里就是普通静态文件，直接拷过去即可：

```bash
cd web && npm run generate
rsync -av .output/public/ user@host:/var/www/ratchet/
```

nginx 只需要一行：

```nginx
location / { root /var/www/ratchet; try_files $uri $uri/ /index.html; }
```

> `try_files` 那一句不能省：Nuxt 生成了静态目录，直接访问 `/verify`
> 时服务器要知道回落到 `verify/index.html`。

## 本地预览

```bash
npx serve web/.output/public
```

---

## 为什么不做成需要服务端的形态

验证页的全部意义是"**结论在收货方的机器上得出**"。
一旦引入服务端，页面上的哈希就可能由别人代算——那正是这个产品要避免的事。
静态站不是省事，是**设计要求**。
