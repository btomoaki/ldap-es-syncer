# ldap-es-syncer Helm Chart

本ディレクトリには、`ldap-es-syncer` を Kubernetes 環境へデプロイするための Helm Chart のソースコードが含まれています。

---

## 🚀 デプロイ手順

本 Chart はパッケージとして公開されていないため、リポジトリをローカルにクローンし、このディレクトリパス（`./deploy/helm/ldap-es-syncer`）を直接指定してデプロイを行います。

### 1. Docker イメージのビルドとプッシュ
アプリケーションイメージをビルドし、お使いのコンテナレジストリ（Docker Hub, ECR, GCR 等）にプッシュします。

```bash
# イメージのビルド
docker build -f build/package/Dockerfile -t your-registry/ldap-es-syncer:1.0.0 .

# レジストリへのプッシュ
docker push your-registry/ldap-es-syncer:1.0.0
```

### 2. values.yaml のカスタマイズと実行モード設定
`values.yaml` を複製して実環境用の設定ファイル（例: `my-values.yaml`）を作成し、接続先の LDAP / Elasticsearch 情報、パスワード、実行モード等を設定します。

#### 実行モードの切り替え
`sync.daemonMode`（真偽値）によって動作リソースを切り替えられます：

* **デーモンモード（常駐実行）**
  `sync.daemonMode: true` に設定すると、常駐型の **`Deployment`** として起動し、`SYNC_INTERVAL` の間隔ごとに定期同期を行います。
* **ワンオフモード（定期バッチ実行）**
  `sync.daemonMode: false` に設定すると、Kubernetes の **`CronJob`** として登録され、`sync.cronSchedule` で指定した任意のスケジュール（Cron 式）で起動して同期サイクルを回します。

### 3. インストールの実行
ローカルの Chart パスを指定し、設定ファイルを適用してデプロイを実行します。

```bash
helm install ldap-es-syncer ./deploy/helm/ldap-es-syncer -f my-values.yaml
```

---

## 🔒 セキュリティとプライベートレジストリの設定

### イメージプルシークレット (imagePullSecrets)
認証が必要なプライベートレジストリからイメージを取得する場合、事前に Kubernetes 側に docker-registry シークレット（例: `regcred`）を作成した上で、`values.yaml` にて以下のように指定します。

```yaml
imagePullSecrets:
  - name: regcred
```

### 🔐 Native ユーザー同期（Kibana ログイン連携）の制約事項
`sync.syncSecurityUsers: true`（Kibana/Elasticsearch Native ユーザーのパスワード同期機能）を有効化する場合、以下の技術的な仕様制限に注意してください。

1. **パスワードハッシュ形式の制限 (bcrypt 必須):**
   - Elasticsearch の Security API（Native User 認証）は、パスワードハッシュとして **bcrypt**（プレフィックス `$2a$`, `$2b$`）のみをサポートしています。
   - 同期元となる LDAP 側のユーザーパスワード属性（`userPassword`）が、`{CRYPT}`（bcrypt スキーマ）でハッシュ化されている必要があります。
   - `SSHA` や `SHA` などの Elasticsearch 非対応ハッシュ形式のユーザーは、同期処理のエラーを防ぐため、自動的に「ログイン不可能なダミーハッシュ」に置換されて同期され、Kibana へのログインは拒否（`401 Unauthorized`）されます。
2. **無償版 (Basic ライセンス) での1段ログイン統一:**
   - 有償サブスクリプション（SSO/LDAP連携機能）を導入せずに、Kibana のログイン画面（1段）に統一して LDAP と同じパスワードでログインさせるには、LDAP 側の設定を `{CRYPT}` (bcrypt) でハッシュ生成するように変更した上で、**各ユーザーに一度パスワードを再設定（更新）してもらう**必要があります。（ハッシュの不可逆性により、既存の SSHA ハッシュから自動変換することはできません）
