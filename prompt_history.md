# Prompt History

## [2026-06-01] ステップ0: ローカル開発環境の構築

### 概要
プロジェクト `ldap-es-syncer` の着工にあたり、コンテナを用いたローカル開発用の検証環境（Docker Compose）を構築。

### 決定事項
- **標準構成への最適化:** Compose V2の標準に則り、設定ファイル名を `compose.yml` に決定。
- **検証環境の検証:** 同期元として OpenLDAP および phpLDAPadmin、同期先として Elasticsearch 8.8.1 および Kibana を選定。
- **接続の簡易化:** ローカル開発での証明書ハンドリングの複雑さを避けるため、Elasticsearch のセキュリティ（SSL/TLS、基本認証）を一時的に無効化。

### 作成・変更ファイル
- `compose.yml` (新規)

---

## [2026-06-01] ステップ1: プロジェクト構造の再定義とConfig設定

### 概要
Clean Architecture（Ports and Adapters）および `GEMINI.md` の指針に基づき、技術に依存しないフォルダ構造の定義と、環境変数から設定を読み込むConfigモジュールの設計を実施。

### 決定事項
- **純粋なレイヤー分離:** 外部技術の名前をパッケージ名から排除し、`domain`, `application`, `infrastructure`, `di` の4層構造を確立。
- **設定の限定注入 (Config Injection):** DIコンテナが各コンポーネントに必要な設定だけを部分的に注入できるよう、`Config` 構造体を `SourceConfig` や `TargetConfig` 等のセグメントに分割。
- **環境変数の管理:** インフラ層（`infrastructure/config`）に設定読み込みロジックを閉じ込め、ドメイン層の純粋性を維持。

### 作成・変更ファイル
- `internal/infrastructure/config/config.go` (新規)

---

## [2026-06-01] ステップ2: ドメインモデルとリポジトリインターフェースの定義

### 概要
ビジネスルールの核となるドメインモデル（User）と、外部接続のための抽象インターフェース（Repository Ports）を定義。

### 決定事項
- **技術に依存しないモデル:** 同期対象となる `User` 構造体を `domain/model` に定義。パスワードの有無による有効/無効化などのビジネスロジックをモデル内に集約。
- **リポジトリの抽象化 (Ports):** データの取得元、保存先を抽象化した `SourceRepository` と `TargetRepository` インターフェースを `domain/repository` に定義し、インフラ層（LDAP/ES）の詳細を完全に隠蔽。

### 作成・変更ファイル
- `internal/domain/model/user.go` (新規)
- `internal/domain/repository/user_repository.go` (新規)
- `internal/domain/model/user_test.go` (新規)

---

## [2026-06-01] ステップ3: アプリケーションユースケースの実装

### 概要
同期元リポジトリからデータを読み込み、同期先リポジトリへ保存するアプリケーション層のユースケース `SyncUserUseCase` の実装と、テストダブルを用いたユニットテストを構築。

### 決定事項
- **インフラ詳細を関知しないデータフロー制御:** インフラの具体的な技術には依存せず、コンストラクタ注入（DI）されたリポジトリ抽象インターフェースを通じてフローを調整する構造を維持。
- **ログ集約によるクラウド最適化 (Log Aggregation):** 転送量・ストレージコスト削減（クラウド最適化）のため、同期ループ内での個別の成功ログ出力を排除。最後に合計処理件数（例: 「Total processed: X/Y users」）を1行で出力する設計とし、標準出力をキャプチャするテストコードでこれを検証。

### 作成・変更ファイル
- `internal/application/usecase/sync_user_usecase.go` (新規)
- `internal/application/usecase/sync_user_usecase_test.go` (新規)

---

## [2026-06-01] ステップ4: インフラ層（LDAP/Elasticsearch具象アダプター）の実装

### 概要
同期元および同期先となる具体的なインフラストラクチャ層（LDAPおよびElasticsearch）の具象リポジトリ（Adapters）を実装。

### 決定事項
- **外部ライブラリの採用:** LDAP接続に `github.com/go-ldap/ldap/v3`、Elasticsearch接続に `github.com/elastic/go-elasticsearch/v8` を採用。セキュリティ安全検証をクリアした上で導入。
- **設定の限定注入 (Config Injection):** クライアント初期化時にConfig全体を渡さず、必要な設定セグメント（`SourceConfig` / `TargetConfig`）のみを限定注入する設計を徹底。
- **インフラ層の冗長ログ排除:** GEMINI.md の「クラウド最適化ルール」に従い、インフラ層の具象リポジトリ内部（接続成功等）での冗長な成功ログ（stdout）を一切排除し、エラー情報のみを上位層へ伝搬させる設計を徹底。

### 作成・変更ファイル
- `internal/infrastructure/ldap/user_repository.go` (新規)
- `internal/infrastructure/elasticsearch/user_repository.go` (新規)

---

## [2026-06-14] ステップ5: ローカル環境変数の管理統一（.envの導入）とElastic命名規則の完全準拠

### 概要
Docker ComposeおよびGoアプリケーションで共通して利用するローカル環境変数をプロジェクトルートの `.env` に一元化し、Elastic stack公式の命名規則（`ELASTIC_PASSWORD`）へ完全に準拠・統一。

### 決定事項
- **`.env` ファイルの一元管理:** Docker Compose と Go アプリケーションの両方で参照可能な単一の `.env` ファイルをプロジェクトルートに作成し、環境変数設定を統合。
- **公式ガイドラインへの完全準拠:** `ES_PASSWORD` を廃止し、公式規則に沿った `ELASTIC_PASSWORD` に一本化。インフラ設定モジュール（`config.go`）での参照も移行。
- **Composeファイルの環境変数化:** `ldap-server`, `phpldapadmin`, `elasticsearch`, `kibana` の4つのコンテナが協調して動作するよう、`.env` から適切にパラメータを注入。

### 作成・変更ファイル
- `.env` (新規)
- `compose.yml` (変更)
- `internal/infrastructure/config/config.go` (変更)
- `internal/infrastructure/config/config_test.go` (変更)
- prompt_history.md (変更)

---

## [2026-06-14] ステップ6: DI層の実装およびアプリケーション起動エントリーポイントの作成

### 概要
Clean Architectureおよび `GEMINI.md` に基づき、依存関係の注入（DI）層とアプリケーションの起動エントリーポイント（main.go）を実装し、エンドツーエンドの実行フローを確立。

### 決定事項
- **DI層による配線管理:** `internal/di` パッケージを新規作成。設定ファイルのロードとインフラアダプター、ユースケースの依存解決をここに集約。
- **インターフェース返却 of the system 徹底:** 各インフラアダプターのコンストラクタ戻り値を具象構造体のポインタからドメイン層のインターフェースに変更し、疎結合化。
- **起動エントリーポイントの実装:** `cmd/main.go` を作成。DIコンテナを初期化してユースケースを取得し、同期を1サイクル実行する仕組みを確立。
- **クラウドネイティブ・ロギングの適用:** 標準ログ出力をライフサイクルイベント（Stdout）と詳細エラー（Stderr）へ適切に切り分け。

### 作成・変更ファイル
- `internal/di/di.go` (新規)
- `cmd/main.go` (新規)
- `internal/infrastructure/ldap/user_repository.go` (変更)
- `internal/infrastructure/elasticsearch/user_repository.go` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-14] ステップ7: デーモンモードとワンオフモードの切り替えおよびシグナルハンドリングの実装

### 概要
同期アプリケーションにおいて、定期実行（`time.Ticker` を用いたデーモンモード）と単発実行（ワンオフモード）の両方を環境変数で制御可能にし、さらに `SIGINT` および `SIGTERM` シグナルを受け取る優雅な停止（Graceful Shutdown）ロジックを実装。

### 決定事項
- **動作モードの切り替え:** `DAEMON_MODE` 環境変数（真偽値、デフォルトは `true`）を導入し、デーモン実行とバッチ実行を切り替えられるように設計。
- **実行間隔の設定:** デーモン実行時の同期間隔を制御する `SyncInterval` 設定（`SYNC_INTERVAL` 環境変数から取得、デフォルト値は1時間など）を追加。
- **クリーンな停止処理 (Graceful Shutdown):** デーモン実行中に `SIGINT` や `SIGTERM` を受信した際、同期処理が実行中であれば完了を待ってから終了する仕組みを導入し、中途半端な同期状態を防ぐ。
- **デーモンループと単発実行:** `cmd/main.go` にて、デーモンモードの場合は `time.Ticker` によるループ処理を、オフの場合は1回のみ実行して終了する処理を実装。
- **クラウドネイティブ・ロギングの遵守:** 起動・停止のライフサイクルイベントと、同期完了時の統計情報のみを最小限の標準出力（Stdout）に出力。

### 作成・変更ファイル
- `cmd/main.go` (変更)
- `.env` (変更)
- `internal/infrastructure/config/config.go` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-14] ステップ8: Elasticsearch インデックスマッピングの自動初期化と構造化ロギングの導入

### 概要
アプリケーション起動時に Elasticsearch のユーザーインデックスの存在チェックとマッピング定義を自動で行う処理を実装。さらに、ロギング機構を Go 標準の構造化ロガー（`log/slog`）に移行し、ライフサイクルや統計情報は stdout、詳細なエラー情報は stderr へ JSON 形式で出力する仕組みを導入。

### 決定事項
- **起動時のインデックス初期化:** Elasticsearch リポジトリのインスタンス化時にターゲットインデックスの存在を確認。存在しない場合は自動的にインデックスを作成し、明示的なフィールドマッピング（ID、Username、Email、IsActive、UpdatedAt）を適用。
- **Go 構造化ロギング (`log/slog`):** 標準の `log` パッケージから Go の構造化ロガーである `log/slog` へ移行。
- **出力ストリームの振り分け (Stdout/Stderr):** `slog` のハンドラーをカスタマイズし、`INFO` レベルは標準出力（stdout）へ、`ERROR` レベルは標準エラー出力（stderr）へ振り分けることで、クラウドネイティブ・ロギングのルールを充足。
- **ログ集約テストの更新:** ユニットテストを更新し、標準ログ出力の代わりに `slog` からの JSON 出力をキャプチャして検証するよう修正。

### 作成・変更ファイル
- `cmd/main.go` (変更)
- `internal/application/usecase/sync_user_usecase.go` (変更)
- `internal/application/usecase/sync_user_usecase_test.go` (変更)
- `internal/infrastructure/elasticsearch/user_repository.go` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-14] ステップ9: 動的環境変数テンプレートの導入とElasticsearch既存データとのリコンシリエーションによる論理削除の実装

### 概要
プロダクションレベルの堅牢性と柔軟性を確保するため、結合テストフェーズに進む前にシステムアーキテクチャを拡張。「LDAPクエリの動的テンプレート化」および「Elasticsearchの既存データとのリコンシリエーションフロー」を導入し、厳密な論理削除（`IsActive = false`）を実現。これにより、アカウントの有効/無効のライフサイクルがLDAPマスターによって完全に制御されるというID管理の原則を遵守する。

### 決定事項
- **動的LDAPフィルターテンプレート:** `LDAP_FILTER` 内のプレースホルダー `{LDAP_ACTIVE_USER}` を `LDAP_ACTIVE_USER` の値で置換する環境変数設定ロジックを追加し、結果を `FinalFilter` として保持。
- **動的属性マッピング:** 動的な属性キー（`LDAP_MAP_UID`, `LDAP_MAP_USERNAME`, `LDAP_MAP_EMAIL`）を設定モジュールにロードし、ドメインの `User` エンティティ構築時に動的に適用。
- **Elasticsearchリコンシリエーション用ポートの追加:** ターゲットリコンシリエーションを可能にするため、`GetAllUserIDs` および `GetUser` インターフェースを `TargetRepository` に追加。
- **リコンシリエーションに基づく非アクティブ化パイプライン:** 同期ユースケースを書き換え、コンパイルされた `FinalFilter` を出力し、LDAPの生存者全員を `IsActive = true` でアップサートし、Elasticsearch上の既存IDと突き合わせて、LDAP上でアクティブでなくなった、または削除されたユーザーを非アクティブ化（`IsActive = false`）する処理を実装。システムユーザー（`elastic`, `kibana_system`）は非アクティブ化の対象から除外。

### 作成・変更ファイル
- `internal/infrastructure/config/config.go` (変更)
- `internal/domain/repository/user_repository.go` (変更)
- `internal/infrastructure/ldap/user_repository.go` (変更)
- `internal/infrastructure/elasticsearch/user_repository.go` (変更)
- `internal/application/usecase/sync_user_usecase.go` (変更)
- `internal/application/usecase/sync_user_usecase_test.go` (変更)
- `.env` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-18] ステップ10: LDAP_SKIP_VERIFY の実装

### 概要
LDAPサーバーとの TLS 接続時における証明書検証のスキップ制御（`LDAP_SKIP_VERIFY`）を実装。

### 決定事項
- **設定値の追加**: `.env` に `LDAP_SKIP_VERIFY=true`（デフォルトは `false`）を追加。
- **Configモジュールの拡張**: `config.go` の `SourceConfig` に `SkipVerify` フィールドを追加し、環境変数のロードロジックを実装。
- **LDAPリポジトリへの適用**: `ldap/user_repository.go` での接続時（`ldap.DialURL`）、TLS接続オプション（`InsecureSkipVerify`）に設定値を適用。

### 作成・変更ファイル
- `.env` (変更)
- `internal/infrastructure/config/config.go` (変更)
- `internal/infrastructure/ldap/user_repository.go` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-18] ステップ11: セーフティガードと取得上限数の環境変数による制御の実装、および環境変数のネームスペース整理（リファクタリング）

### 概要
誤操作防止のためのセーフティガード（アボート閾値）および Elasticsearch 取得最大件数（上限）を環境変数で制御可能にし、それぞれ同期処理および検索クエリに適用。また、環境変数の命名および `.env` 内の配置をネームスペース（`APP_`, `SYNC_`, `LDAP_`, `ES_`）を意識した構成にリファクタリングする。

### 決定事項
- **環境変数のリファクタリングと追加**:
  - アプリ全体および同期処理の挙動を `Application Config` セクションにまとめ、プレフィックスを `APP_` または `SYNC_` に整理：
    - `APP_ENV=development`
    - `APP_LOG_LEVEL`（旧: `LOG_LEVEL`、デフォルト: `info`）
    - `SYNC_DAEMON_MODE`（旧: `DAEMON_MODE`、デフォルト: `true`）
    - `SYNC_INTERVAL`（既存、デフォルト: `1h`）
    - `SYNC_MIN_USERS`（新規追加、デフォルト: `1`）
  - LDAP 関連の変数を `LDAP Configuration` セクションに整理：
    - `LDAP_SKIP_VERIFY`（新規追加、デフォルト: `false`）を同セクションへ移動。
  - Elasticsearch 関連の変数を `Elasticsearch & Kibana Connection` セクションに整理：
    - `ES_MAX_RESULTS`（新規追加、デフォルト: `1000`）を同セクションへ移動。
- **Configモジュールの拡張**:
  - 新しい環境変数名に対応するロード処理を `config.go` に実装。
  - `AppConfig` に `SyncMinUsers int` フィールドを追加。
  - `TargetConfig` に `MaxResults int` フィールドを追加。
  - テストコードの更新。
- **セーフティガードの実装**:
  - `sync_user_usecase.go` にて、LDAPから取得したユーザー数が `SyncMinUsers` 未満の場合、誤設定や未準備による全ユーザー誤削除を防止するため、同期および論理削除処理を一切行わずに安全にアボート（エラー終了）するガードを追加。
  - ユースケースのテストコードを更新し、ガードがトリガーされるケースをテスト。
- **取得上限の適用**:
  - `EsUserRepository` 構造体に `maxResults int` フィールドを追加。
  - `GetAllUserIDs` メソッドの検索クエリの `"size"` パラメータに `MaxResults` を適用。これによりESからの一括取得数を最大1,000件に制限し、これを超える規模での同期運用は非推奨（動作対象外）とする。

### 作成・変更ファイル
- `.env` (変更)
- `internal/infrastructure/config/config.go` (変更)
- `internal/infrastructure/config/config_test.go` (変更)
- `internal/application/usecase/sync_user_usecase.go` (変更)
- `internal/application/usecase/sync_user_usecase_test.go` (変更)
- `internal/di/di.go` (変更)
- `internal/infrastructure/elasticsearch/user_repository.go` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-21] ステップ12: シグナルハンドリングの強化とコンテキスト連動（Graceful Shutdown の高度化）の実装

### 概要
デーモンモードおよびワンオフモードにおいて、シグナル（`SIGINT`/`SIGTERM`）の受信に連動してコンテキストを即座にキャンセルさせ、同期実行中（ループ処理やリポジトリ操作）であっても迅速かつ安全に処理をアボート（Graceful Shutdown）する仕組みを実装。

### 決定事項
- **シグナルコンテキストの導入 (`signal.NotifyContext`):** `cmd/main.go` のデーモンおよびワンオフの起動処理において、シグナル検知時に自動でキャンセルされるコンテキスト `ctx` を生成し、同期処理へ受け渡す。
- **実行中処理の即時中断:**
  - `SyncUserUseCase.Execute` の実行ループ（生存者のUpsertループ、および既存ESデータとの論理削除の突き合わせループ）の各反復で定期的に `ctx.Err()` をチェックし、コンテキストがキャンセルされている場合は安全にアボート（エラー終了）する設計に変更。
  - リポジトリの各データアクセス処理（LDAPのFetch、ESのSave/Get/Search）に対してもコンテキストを確実に伝搬させ、タイムアウトやキャンセルが下位レイヤーまで連動するように実装。
- **ワンオフモードへの適用:** デーモンモードだけでなく、ワンオフモードの実行においてもシグナルハンドリングを導入し、中断時のデータ不整合リスクを低減。

### 作成・変更ファイル
- `cmd/main.go` (変更)
- `internal/application/usecase/sync_user_usecase.go` (変更)
- `prompt_history.md` (変更)

---

## [2026-06-21] ステップ13: Prometheus による監視・可観測性 (Monitoring & Observability) の実装

### 概要
Prometheus による同期処理の監視に対応するため、メトリクス用の抽象ポートの定義、Prometheus SDK を用いた具象アダプター、No-op モック実装、および動作モード（デーモン/バッチ）に応じたメトリクス収集・公開方式の切り替え（Pull型/Push型）を実装。

### 決定事項
- **環境変数の追加:** 監視の有効化、ポート、プッシュ先を指定する `METRICS_ENABLED`（デフォルト: `false`）、`METRICS_PORT`（デフォルト: `8080`）、`METRICS_PUSHGATEWAY_URL`（デフォルト: 空、例: `http://localhost:9091`）を定義。
- **MetricsRepository 抽象ポートの定義 (Domain層):** 同期処理時間、処理ユーザー数、アクティブユーザー数、同期ステータス（成功/失敗）を記録するためのインターフェース `MetricsRepository` を `internal/domain/repository` に定義。
- **Prometheus 具象アダプターの実装 (Infrastructure層):**
  - `github.com/prometheus/client_golang` を用いて、ヒストグラム（同期時間）、カウンター（累積処理ユーザー数）、ゲージ（現在のアクティブユーザー数、直近の同期成否）の各メトリクスを定義・更新する `PrometheusMetricsRepository` を実装。
  - 監視が無効である場合やユニットテストのために、何も処理を行わない `NoopMetricsRepository` を実装。
- **動作モードに応じたメトリクス収集・公開の統合:**
  - **デーモンモード（常駐）**: `MetricsEnabled` が真かつデーモンモードでの起動時、`cmd/main.go` で `/metrics` エンドポイントを公開する HTTP サーバーをバックグラウンドで起動し、Graceful Shutdown 時に `Shutdown(ctx)` で安全にソケットをクローズする（Pull型）。
  - **バッチ（ワンオフ）モード**: `MetricsEnabled` が真のとき、同期処理の実行完了時に Prometheus Pushgateway にメトリクスを送信（プッシュ）して終了する（Push型）。
- **ユースケースの拡張:** `SyncUserUseCase` に `MetricsRepository` を注入し、同期処理の開始・終了時に適切に各種メトリクスを記録するように実装。

### 作成・変更ファイル
- `.env` (変更)
- `internal/infrastructure/config/config.go` (変更)
- `internal/domain/repository/metrics_repository.go` (新規)
- `internal/infrastructure/prometheus/metrics_repository.go` (新規)
- `internal/infrastructure/prometheus/noop_metrics_repository.go` (新規)
- `internal/application/usecase/sync_user_usecase.go` (変更)
- `internal/application/usecase/sync_user_usecase_test.go` (変更)
- `internal/di/di.go` (変更)
- `cmd/main.go` (変更)
- `prompt_history.md` (変更)