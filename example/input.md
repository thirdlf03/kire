# Chat API 設計ドキュメント

このドキュメントは、リアルタイムチャットアプリケーションの API 設計をまとめたものである。認証、メッセージング、通知の 3 つの主要コンポーネントについて記述する。

## 認証

### ユーザー登録

新規ユーザーはメールアドレスとパスワードで登録する。パスワードは bcrypt でハッシュ化し、平文では保存しない。

```
POST /api/v1/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "securePassword123",
  "display_name": "Alice"
}
```

レスポンスとして JWT アクセストークンとリフレッシュトークンを返す。アクセストークンの有効期限は 15 分、リフレッシュトークンは 7 日間である。

### トークン更新

アクセストークンの有効期限が切れた場合、リフレッシュトークンを使って新しいトークンペアを取得する。

```
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGciOiJSUzI1NiIs..."
}
```

リフレッシュトークンはローテーション方式を採用する。トークン更新時に古いリフレッシュトークンは無効化され、新しいペアが発行される。これにより、トークンが漏洩した場合のリスクを軽減する。

### OAuth 連携

Google と GitHub の OAuth 2.0 によるソーシャルログインにも対応する。OAuth フローでは PKCE（Proof Key for Code Exchange）を必須とし、認可コード横取り攻撃を防ぐ。

## メッセージング

### メッセージ送信

認証済みユーザーは WebSocket 経由でリアルタイムにメッセージを送信できる。

```json
{
  "type": "message.send",
  "channel_id": "ch_abc123",
  "content": "Hello, World!",
  "attachments": []
}
```

メッセージは Kafka キューに投入され、非同期で永続化される。これにより、DB の書き込み遅延がリアルタイム配信に影響しない。メッセージには一意の ID（ULID）が付与され、順序保証とべき等性を実現する。

### チャンネル管理

チャンネルは 3 種類ある:

- **パブリック**: 全ユーザーが参加可能
- **プライベート**: 招待制。管理者がメンバーを追加・削除する
- **ダイレクト**: 1 対 1 の会話。自動的にプライベートとして扱われる

チャンネルの作成・更新・削除は REST API で行う。

```
POST /api/v1/channels
GET /api/v1/channels/:id
PUT /api/v1/channels/:id
DELETE /api/v1/channels/:id
```

チャンネルごとにメッセージ保持期間を設定でき、デフォルトは 90 日間である。保持期間を過ぎたメッセージはアーカイブストレージに移動される。

### メッセージ検索

全文検索は Elasticsearch を使用する。日本語の形態素解析には kuromoji プラグインを導入し、検索精度を高めている。

検索 API はページネーションとフィルタリングをサポートする:

```
GET /api/v1/search?q=keyword&channel_id=ch_abc123&from=2025-01-01&limit=20
```

検索インデックスはメッセージ投入時に非同期で更新される。Kafka Consumer が Elasticsearch への書き込みを担当し、メイン処理パスとは分離されている。

## 通知システム

### プッシュ通知

メンション（@user）やダイレクトメッセージを受信した際に、モバイル端末へプッシュ通知を送信する。Firebase Cloud Messaging（FCM）と Apple Push Notification Service（APNs）の両方に対応する。

通知の送信判定ロジック:

1. 受信者がオンライン（WebSocket 接続中）なら通知をスキップ
2. 受信者のミュート設定を確認
3. 通知レート制限（同一チャンネルから 5 分間に最大 3 件）を適用
4. 条件を満たせば通知キューに投入

### 通知設定

ユーザーごとに細かい通知設定が可能:

| 設定項目 | デフォルト | 説明 |
|---------|-----------|------|
| メンション通知 | ON | @user で通知する |
| DM 通知 | ON | ダイレクトメッセージで通知する |
| チャンネル通知 | OFF | チャンネルの全メッセージで通知する |
| おやすみモード | OFF | 指定時間帯は通知しない |
| サウンド | ON | 通知音を鳴らす |

設定は REST API で変更する:

```
GET /api/v1/users/me/notification-settings
PUT /api/v1/users/me/notification-settings
```

### メール通知

72 時間以上未読のメンションがある場合、ダイジェストメールを送信する。メール送信は Amazon SES を使用し、送信レートは 1 ユーザーあたり 1 日 1 通に制限する。

メールテンプレートは MJML で記述し、レスポンシブデザインに対応している。

## データベース設計

### スキーマ概要

主要テーブル:

- `users`: ユーザー情報（email, display_name, avatar_url, created_at）
- `channels`: チャンネル情報（name, type, retention_days, created_by）
- `messages`: メッセージ本体（channel_id, sender_id, content, created_at）
- `channel_members`: チャンネル参加者（channel_id, user_id, role, joined_at）
- `notification_settings`: 通知設定（user_id, mention, dm, channel, quiet_hours）

### パーティショニング

`messages` テーブルは `created_at` による月次パーティショニングを適用する。これにより、古いメッセージのアーカイブと削除が効率的になる。

パーティション管理は pg_partman で自動化し、3 ヶ月先までのパーティションを事前作成する。

### インデックス戦略

頻出クエリに対して以下のインデックスを作成する:

```sql
-- チャンネル内メッセージの時系列取得
CREATE INDEX idx_messages_channel_created ON messages (channel_id, created_at DESC);

-- ユーザーのメンション検索
CREATE INDEX idx_messages_mentions ON messages USING GIN (mentions);

-- 未読メッセージカウント
CREATE INDEX idx_messages_unread ON messages (channel_id, created_at)
  WHERE read_at IS NULL;
```

部分インデックス（`WHERE read_at IS NULL`）を使うことで、既読メッセージを含まない軽量なインデックスを実現している。

## デプロイメント

### インフラ構成

本番環境は AWS 上に構築する:

- **API サーバー**: ECS Fargate（オートスケーリング）
- **WebSocket**: ECS + NLB（スティッキーセッション）
- **データベース**: Aurora PostgreSQL（マルチ AZ）
- **キャッシュ**: ElastiCache Redis（クラスターモード）
- **メッセージキュー**: Amazon MSK（Managed Kafka）
- **検索**: Amazon OpenSearch Service
- **CDN**: CloudFront（静的アセット配信）

### CI/CD パイプライン

GitHub Actions でビルド・テスト・デプロイを自動化する。

```yaml
main ブランチへの push:
  1. ユニットテスト + E2E テスト実行
  2. Docker イメージビルド & ECR プッシュ
  3. ステージング環境へデプロイ
  4. スモークテスト実行
  5. 承認ゲート（手動承認）
  6. 本番環境へカナリアデプロイ（10% → 50% → 100%）
```

ロールバックは ECS のサービスリビジョンを前のバージョンに戻すだけで完了する。

### 監視

Datadog を使用して以下のメトリクスを監視する:

- API レスポンスタイム（P50, P95, P99）
- WebSocket 接続数
- Kafka Consumer ラグ
- DB コネクションプール使用率
- エラーレート（5xx）

アラート閾値は P99 レスポンスタイム 500ms 超過、エラーレート 1% 超過で PagerDuty に通知する。
