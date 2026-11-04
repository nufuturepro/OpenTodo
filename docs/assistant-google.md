---
title: Google accounts for the assistant
summary: Use Google Gemini with a person's own Google account instead of an API key: the owner's OAuth client, what Google bills, where the tokens live, and how to disconnect.
audience: user
section: Reference
specs: [in-app-assistant]
screens: []
order: 83
---
# Google accounts for the assistant

People can use [Google Gemini with their own Google account](assistant-settings.md#use-your-own-provider-and-key) instead of pasting an API key. The server owner sets up one OAuth client at Google; everyone else then selects **Connect Google account** and signs in. The assistant asks Google for `openid`, `email` and `generative-language.retriever` only. It does not use the key-less sign-in of Google's own Gemini CLI, which Google's terms do not allow third-party software to use.

## At Google, once

1. In [Google Cloud](https://console.cloud.google.com/), enable the **Generative Language API** for a project.
2. Create an **OAuth client** of type *Web application* and add the redirect address that the Assistant card shows you. It ends in `/api/v1/assistant/google/callback`, on the address people reach the server at.
3. While the client is in *Testing*, add each person under **Test users**. A public instance needs Google's verification of the `generative-language.retriever` scope.

## In OpenTodo, once

**Settings → Integrations → Assistant → Configure** shows **Google accounts** under Google Gemini: the **Client ID**, the **Client secret** and, optionally, the **Google Cloud project**, with **Save Google client** and whether the client is set up. The secret goes to the server's secrets file, is never shown again, and is in no backup or export. Nothing is offered to members until the owner also turns on **Let members bring their own key**.

## For each person

**Settings → Integrations → Assistant** then offers **Connect Google account**, with an optional **Google Cloud project** for where their calls are billed. They sign in at Google, allow OpenTodo, and land back connected; the card shows the connected address. **Disconnect Google account** deletes the stored access from the server; removing OpenTodo at [myaccount.google.com/permissions](https://myaccount.google.com/permissions) withdraws it at Google too.

A connected account answers that person's turns whenever Google Gemini is the provider, whether they chose it themselves or it is the owner's default. An API key they saved is used before the connected account, as the card says.

## Tokens

The refresh token is in the server's secrets file, and the connected address, project and state are in the assistant's table. Access tokens are minted from the refresh token, kept in memory for their hour and never written to disk or served to a browser. Requests go from the server to `generativelanguage.googleapis.com`, and the same [list of what may be sent](assistant-privacy.md#before-your-first-message) applies. A request that arrives with a token Google has expired is retried once with a fresh one.

## Costs

Calls on a Google account are billed to the Google Cloud project named for the client, unless the person named their own project when connecting. Google documents neither a per-user free allowance nor per-user billing on this path, so the project field is how the bill is aimed. Tokens on a Google account count as the person's own credential: they are shown in **Settings → Integrations → Assistant** and do not count toward the owner's monthly allowance unless the owner counts members' own keys.

## When it does not work

- `google_not_configured`: the owner has no complete client, so nobody can connect.
- **Google refused this connection; connect again**: the grant was revoked or expired, here or at Google. Until it is connected again, that person's turns answer `provider_auth`.
- **Cancelled at Google**, or a sign-in that took longer than ten minutes to come back, leaves the page as it was; start again from **Connect Google account**.
- A project id must look like `my-project-123456`; anything else is refused before Google is asked.

The routes are in the [API reference](api.md#in-app-assistant).
