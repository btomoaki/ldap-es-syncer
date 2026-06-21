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

---

## [2026-06-21] ステップ14: 監視運用テスト環境（Pushgateway, Prometheus, Grafana）の整備

### 概要
監視の運用テストのため、`compose.yml` に Prometheus、Pushgateway、Grafana を追加。Prometheus のアラート定義および Grafana のダッシュボード設定・プロビジョニング構成を定義し、連携可能にする。

### 決定事項
- **Composeサービスの追加**: Prometheus (ポート `9090`), Pushgateway (ポート `9091`), Grafana (ポート `3000`) を `compose.yml` に追加。
- **ポート競合の回避**: ホスト側の phpLDAPadmin のポート（`8080`）との競合を避けるため、Go アプリ側の Prometheus メトリクス公開ポートを `8080` から `8081` へ変更。
- **アラート検証の簡易化（テスト用ルールの導入）**: 運用テスト時に何時間も待たずにアラートの発火テストができるよう、本番用の `alert.rules`（遅延検知2時間等）とは別に、即時発報・短時間で検知するテスト用ルールファイル `alert-test.rules`（遅延検知2分等）を作成。
- **マウントファイルの環境変数化**: `.env` に `ALERT_RULES_FILE` を導入し、`compose.yml` の Prometheus マウント設定で動的にルールファイルを切り替えられるように設計。
- **Prometheusの設定定義**:
  - `prometheus.yml` にて `prometheus` 自身および `pushgateway` のスクレイプ設定（`honor_labels: true` 付き）を定義。
- **Grafanaのプロビジョニング自動化**:
  - データソース (`datasource.yml`) に Prometheus を自動登録。
  - ダッシュボードのプロビジョニング (`dashboard.yml` と JSON定義ファイル) により、起動時に ldap-es-syncer 監視用ダッシュボードを自動で読み込み表示可能にする。

### 作成・変更ファイル
- `.env` (変更)
- `compose.yml` (変更)
- `prometheus/prometheus.yml` (新規)
- `prometheus/alert.rules` (新規)
- `prometheus/alert-test.rules` (新規)
- `grafana/provisioning/datasources/datasource.yml` (新規)
- `grafana/provisioning/dashboards/dashboard.yml` (新規)
- `grafana/dashboards/ldap_es_syncer_dashboard.json` (新規)
- `prompt_history.md` (変更)

---

## [2026-06-21] ステップ15: テスト・検証用アセットのディレクトリ整理と Docker 構成の追加

### 概要
テスト・検証環境で使用する各種設定ファイルがプロジェクトルートに散在して散らかってきたため、`test/` ディレクトリを導入して整理整頓を行う。また、同期アプリケーションのコンテナイメージ構築のために、マルチステージビルドによる `Dockerfile` およびローカル検証用のビルドスクリプトを追加し、Docker環境を整備する。

### 決定事項
- **検証アセット用フォルダの定義**: `test/monitoring/` ディレクトリを作成し、監視関係の設定ファイルをすべてそこに移動。
  - `prometheus/` -> `test/monitoring/prometheus/`
  - `grafana/` -> `test/monitoring/grafana/`
- **Compose設定のパス更新**: `compose.yml` 内の Prometheus および Grafana のマウントボリュームパスを、移動先の `test/monitoring/...` パスに書き換えて整合性を保つ。
- **Dockerfile / .dockerignore の配置**: 標準Goプロジェクトレイアウト（Standard Go Project Layout）に準拠し、プロジェクトルートをクリーンにするため `build/package/` ディレクトリを新設し、`Dockerfile` と `.dockerignore` をそこに配置。
- **ビルド・検証スクリプトの作成**: `scripts/build-docker.sh` を追加し、`-f build/package/Dockerfile` を指定してビルドを行うことで、ローカルレジストリへのプッシュや Kind クラスラーへのロード手順を自動化・案内できるように整備。

### 作成・変更ファイル
- `compose.yml` (変更)
- `build/package/Dockerfile` (新規)
- `build/package/.dockerignore` (新規)
- `scripts/build-docker.sh` (新規)
- `test/monitoring/prometheus/prometheus.yml` (新規)
- `test/monitoring/prometheus/alert.rules` (新規)
- `test/monitoring/prometheus/alert-test.rules` (新規)
- `test/monitoring/grafana/provisioning/datasources/datasource.yml` (新規)
- `test/monitoring/grafana/provisioning/dashboards/dashboard.yml` (新規)
- `test/monitoring/grafana/dashboards/ldap_es_syncer_dashboard.json` (新規)
- `prometheus/prometheus.yml` (削除)
- `prometheus/alert.rules` (削除)
- `prometheus/alert-test.rules` (削除)
- `grafana/provisioning/datasources/datasource.yml` (削除)
- `grafana/provisioning/dashboards/dashboard.yml` (削除)
- `grafana/dashboards/ldap_es_syncer_dashboard.json` (削除)
- `prompt_history.md` (変更)

---

## [2026-06-21] ステップ16: ドキュメント作成（README.md）と検証データ（bootstrap.ldif）の整備

### 概要
新規開発環境への移行や結合テスト実行のために、アプリケーション概要・環境構築手順をまとめた `README.md` の作成、およびローカル OpenLDAP 起動時に自動でインポートされるテスト用の初期ユーザーデータ（`test/ldap/bootstrap.ldif`）の整備を行う。

### 決定事項
- **README.md の作成**:
  - アーキテクチャの概要（クリーンアーキテクチャ、Ports & Adapters構成、デーモン/ワンオフ動作モード）の説明。
  - クイックスタート手順（Docker Composeによるミドルウェア起動 $\rightarrow$ Goアプリ起動）の明記。
  - 全環境変数のネームスペース（`APP_`, `SYNC_`, 外部ミドルウェア公式変数等）による分類と詳細な説明。
- **bootstrap.ldif の整備**:
  - `test/ldap/bootstrap.ldif` を新設し、同期用テストユーザー（`john.doe`, `jane.smith` 等）を定義。さらに、グループ・ロール設計に基づき `ou=groups` 配下に `app_admin`, `app_user`, `app_readonly` のグループ（`groupOfUniqueNames`）を定義し、ユーザーDNを `uniqueMember` 属性で紐付ける。
  - `compose.yml` 内の OpenLDAP サービスにボリュームマウント設定を追加し、コンテナの初期化時にテストデータが自動的にロードされるよう設定。
- **LDAP_FILTER のグループ対応**:
  - `.env` の `LDAP_FILTER` を、上記グループ（ロール）に所属するユーザーのみを同期対象とするフィルター（`memberOf` 属性を用いたOR条件）に更新し、同期スコープを特定システムに関連するユーザーに限定する。

### 作成・変更ファイル
- `prompt_history.md` (変更)
- `README.md` (新規)
- `test/ldap/bootstrap.ldif` (新規)
- `compose.yml` (変更)
- `.env` (変更)

---

## [2026-06-21] ステップ17: デプロイ・パッケージング関連 (Deployment & Packaging) - Helm Chart の作成

### 概要
Kubernetes 環境へアプリケーションをデプロイ・運用できるように、アプリケーションの Helm Chart（Deployment, Secret, ConfigMap, CronJob 構成等）の整備を行う。また、プライベートレジストリ等に対応するため `image.registry` パラメータを導入し、`values.yaml` から Docker レジストリを動的に指定できるように拡張する。

### 決定事項
- **Helm Chart 構成の標準化**: `deploy/helm/ldap-es-syncer` ディレクトリ内に標準的な Helm Chart 構造を構築。
- **動作モードに応じたテンプレート対応**:
  - `SYNC_DAEMON_MODE` が `true` の場合は常駐型（`Deployment`）で動作。
  - `SYNC_DAEMON_MODE` が `false` の場合は定期実行バッチ型（`CronJob`）で動作するよう、`values.yaml` の値に応じて自動でリソース定義を切り替える設計。
- **環境変数のマッピングと機密情報管理**:
  - `.env` で使用している各設定項目を `values.yaml` で管理。
  - パスワードなどの機密情報は Kubernetes `Secret` を通じて安全にコンテナへ注入。
  - その他の設定値は `ConfigMap` を通じてマッピング。
- **Dockerイメージ取得元（レジストリ）設定の追加**:
  - `values.yaml` の `image` セクションに `registry` キーを追加（デフォルト値は空 `""`）。
  - `deployment.yaml` および `cronjob.yaml` 内のイメージ参照部（`image:`）にて、`image.registry` が指定されている場合はレジストリ名を含めてフルパス（`registry/repository:tag`）とし、未指定の場合はリポジトリ名のみ（`repository:tag`）にする条件分岐ロジックを実装。

### 作成・変更ファイル
- `prompt_history.md` (変更)
- `deploy/helm/ldap-es-syncer/Chart.yaml` (新規)
- `deploy/helm/ldap-es-syncer/values.yaml` (新規)
- `deploy/helm/ldap-es-syncer/templates/configmap.yaml` (新規)
- `deploy/helm/ldap-es-syncer/templates/secret.yaml` (新規)
- `deploy/helm/ldap-es-syncer/templates/deployment.yaml` (新規)
- `deploy/helm/ldap-es-syncer/templates/cronjob.yaml` (新規)
- `deploy/helm/ldap-es-syncer/templates/_helpers.tpl` (新規)

---

## [2026-06-21] ステップ18: 結合テスト（GoによるE2E統合テストおよびKindハイブリッドテスト）の実装

### 概要
ローカルで起動している OpenLDAP および Elasticsearch に対するGo言語ベースのE2E統合テスト、およびKind上で同期アプリを動かしてホスト側のミドルウェアに接続するKubernetesハイブリッド結合テストの整備。

### 決定事項
- **GoによるE2E統合テストの実装**:
  - `test/integration/integration_test.go` を作成し、ローカルの OpenLDAP と Elasticsearch に対して実際の接続と同期、論理削除、システムユーザー保護をテスト。
  - Elasticsearch の built-in ユーザーの `metadata._reserved: true` フラグを検証するコードをテスト内に整備。
  - テストの実行・ラップを行うシェルスクリプト `test/integration/run-integration-test.sh` を作成。
- **Kind上でのハイブリッド結合テストの実装**:
  - `test/integration/test-k8s-hybrid.sh` を作成し、アプリのコンテナイメージビルド、Kind へのロード、Helm によるデプロイ、CronJob からの Job 起動、ログ検証、アンインストールまでの一連のシナリオを自動化。
  - Kind ポッドからホストの OpenLDAP / Elasticsearch に接続するため、`hostAliases` に Kind ネットワークのゲートウェイ（`172.18.0.1`）をマッピングする設定を適用。

### 作成・変更ファイル
- `prompt_history.md` (変更)
- `test/integration/integration_test.go` (新規)
- `test/integration/run-integration-test.sh` (新規)
- `test/integration/test-k8s-hybrid.sh` (新規)

---

## [2026-06-21] ステップ19: 結合テストに関するドキュメント（README.md）の追加・更新

### 概要
新しく追加されたGo言語ベースのE2E統合テスト、およびKindを用いたKubernetesハイブリッド結合テストの実行手順・詳細説明を `README.md` に追加。

### 決定事項
- **README.md の更新**:
  - `README.md` に「結合テスト（Integration Testing）」セクションを新設。
  - ローカルGoテスト（`run-integration-test.sh`）および Kubernetesハイブリッドテスト（`test-k8s-hybrid.sh`）の前提条件、実行手順、および挙動の解説を追加。

### 作成・変更ファイル
- `prompt_history.md` (変更)
- `README.md` (変更)

---

## [2026-06-21] ステップ20: Dry-RunモードおよびKibanaロールとLDAPグループのマッピング機能の実装とテスト整備

### 概要
実際の書き込み（保存・論理削除）をスキップしてログ出力のみを行う Dry-Run モード、および LDAP グループ名と Elasticsearch/Kibana ロール名を紐付けるマッピング機能を実装。また、コンストラクタの変更等によって壊れていたテストコード（単体テストおよび結合テスト）を全面的に修正・拡充する。

### 決定事項
- **Dry-Runモードの実装**: 
  - `SYNC_DRY_RUN`（デフォルト: `false`）を環境変数から読み込む。
  - Dry-Runが有効な場合、保存や論理削除の書き込みは行わず、ログ出力（`[Dry-Run]` プレフィックス）のみを行う。
  - Prometheusのメトリクス（処理件数、所要時間、アクティブユーザー数、同期ステータス）への記録も一切行わない。
- **ロール・グループマッピング機能の追加**:
  - LDAPの `memberOf` 属性からグループ名（CN値）を抽出し、ドメインモデル `User.Roles` に代入。
  - Elasticsearch の Security Role Get API (`RoleExists`) を用いて、各グループ名と一致するロールがES上に存在するか確認。
  - ロールが存在する場合はユーザーに割り当て、存在しない（またはAPIが対応していない等でエラーとなった）場合は警告（Warn）ログを出力して割り当てをスキップし、ユーザーアカウントの同期自体は継続する。
- **テストコードの修復と拡充**:
  - `NewSyncUserUseCase` の引数追加に伴い、ビルドエラーとなっていた `sync_user_usecase_test.go` のテストケースに `dryRun` パラメータを追加。
  - `mockTargetRepository` に `RoleExists` のモック動作（ロール存在有無を検証できるように）を追加。
  - Dry-Run モードの動作（書き込みスキップとメトリクス非記録）を検証するテストケースを追加。
  - ロールマッピングと警告ログの出力を検証するテストケースを追加。
  - 結合テスト（`integration_test.go`）なども新コンストラクタ引数に追随させる。

### 作成・変更ファイル
- `prompt_history.md` (変更)
- `internal/application/usecase/sync_user_usecase_test.go` (変更)
- `test/integration/integration_test.go` (変更)
- `internal/di/di.go` (変更)

---

## [2026-06-21] ステップ21: Kibana/Elasticsearch ログインユーザー（Native ユーザー）と LDAP パスワードハッシュの同期機能の実装

### 概要
通常のデータインデックスへの同期に加え、Elasticsearch の Security API（Native User API）を用いて Kibana ログインアカウントを同期する機能を実装。さらに LDAP から取得した `userPassword` ハッシュ値を成形して Elasticsearch に流し込むことで、無償版（Basicライセンス）の範囲内でのパスワードログイン連携を実現する。また、テスト用パスワードの頑健性向上として、`admin` などの安易な文字列の排除と記号（`-`）・数値を含めた一意のパスワード設計（`usr-crypt-pass1`〜`usr-sha-pass4`）を適用する。

### 決定事項
- **Nativeユーザー同期モードの追加**:
  - 設定に `SYNC_SECURITY_USERS`（デフォルト: `false`）環境変数を追加。これが `true` の場合、通常のインデックス同期に加えて、Elasticsearch 上の Native ユーザーアカウントも同期する。
- **UserモデルとLDAPリポジトリの拡張**:
  - `model.User` に `PasswordHash string` フィールドを追加。
  - LDAPリポジトリで取得した `userPassword` 属性（ハッシュ値）を `User.PasswordHash` に代入。
  - パスワードハッシュのスキーマプレフィックス（例: `{CRYPT}`）がある場合、Elasticsearch が解釈可能な形式（bcrypt等のプレフィックス）に成形して保持するロジックを実装。
- **ESリポジトリ（TargetRepository）の拡張**:
  - `TargetRepository` に `SaveSecurityUser` メソッドを追加。
  - Elasticsearch の `PUT /_security/user/{username}` API を用いて、ユーザー名、パスワードハッシュ、ロールを同期。
- **同期ユースケースの拡張**:
  - `syncUserUseCase` で `SYNC_SECURITY_USERS` 設定が有効な場合、通常のインデックス同期ループ内（Upsertおよび論理削除ループ内）で `SaveSecurityUser` や非アクティブ化を並行して呼び出す。
  - システムアカウント（`elastic`, `kibana_system` 等）は Security ユーザー同期の対象からも除外・保護する。
- **テスト用パスワードの一意化・頑健化**:
  - 各検証用ユーザーのパスワードから `admin` などの安易な文字列を排除し、記号（`-`）および数値を含めた一意なパスワード（`usr-crypt-pass1`〜`usr-sha-pass4`）に変更。それに伴い、LDAPサーバー初期化ファイル（`bootstrap.ldif`）のハッシュ値、および結合テストコード（`integration_test.go`）の認証情報を更新。

### 作成・変更ファイル
- `prompt_history.md` (変更)
- `internal/infrastructure/config/config.go` (変更)
- `internal/domain/model/user.go` (変更)
- `internal/domain/repository/user_repository.go` (変更)
- `internal/infrastructure/elasticsearch/user_repository.go` (変更)
- `internal/application/usecase/sync_user_usecase.go` (変更)
- `.env` (変更)
- `test/ldap/bootstrap.ldif` (変更)
- `test/integration/integration_test.go` (変更)