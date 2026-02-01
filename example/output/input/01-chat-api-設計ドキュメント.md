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

