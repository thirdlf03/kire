<!-- context: Chat API 設計ドキュメント > 通知システム > 通知設定 -->

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

