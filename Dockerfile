# syntax=docker/dockerfile:1

# 公開デモ用の単一イメージ。画面、Goサーバー、なでしこ実行系を1つにまとめる。
# 版は scripts/bootstrap-wsl.sh と揃える。ずれた場合は docs/deployment-cloud-run.md を更新する。
FROM --platform=linux/amd64 golang:1.26.0-bookworm AS build

# 取得物は版とsha256で固定する。実行時の取得は行わない。
ARG GONAKO_VERSION=3.8.4
ARG GONAKO_SHA256=1e693de3144cd0199cbc87d858d60d03ee603e1939be53ffc2365481f451b514
ARG NADESIKO_VERSION=3.8.1
ARG NADESIKO_SHA256=749ea6d58f9e4a45857c040a5f1ce2f7e468a6881f5be38e78b6bdfd4cbf6a32

RUN apt-get update \
  && apt-get install -y --no-install-recommends unzip \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# サーバー側なでしこ実行系。Goサーバーがsubprocessとして起動する。
RUN curl -fsSL -o /tmp/gonako.zip \
      "https://github.com/kujirahand/nadesiko3go/releases/download/${GONAKO_VERSION}/gonako-${GONAKO_VERSION}-linux-amd64.zip" \
  && echo "${GONAKO_SHA256}  /tmp/gonako.zip" | sha256sum -c - \
  && unzip -q /tmp/gonako.zip -d /tmp/gonako \
  && install -D -m 0755 /tmp/gonako/gonako /out/bin/gonako \
  && rm -rf /tmp/gonako.zip /tmp/gonako

# 画面とブラウザランタイム。web/vendorは.dockerignoreで除外し、ここで取り直す。
COPY web /out/web
RUN mkdir -p /out/web/vendor \
  && curl -fsSL -o /out/web/vendor/wnako3.js \
      "https://cdn.jsdelivr.net/npm/nadesiko3@${NADESIKO_VERSION}/release/wnako3.js" \
  && echo "${NADESIKO_SHA256}  /out/web/vendor/wnako3.js" | sha256sum -c -

# ルールと棋譜ハーネス。gonakoがそのまま実行する。
COPY rules /out/rules

COPY go.mod ./
COPY server ./server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
      go build -trimpath -ldflags "-s -w" -o /out/nadesiko-reversi-server ./server

FROM --platform=linux/amd64 debian:bookworm-slim AS runtime

# 棋譜再生が一時ファイルを書き、gonakoをsubprocessとして起動するため、
# scratchやstaticではなく最小のDebianを使う。
RUN useradd --system --uid 10001 --shell /usr/sbin/nologin demo

WORKDIR /app
COPY --from=build /out/nadesiko-reversi-server /app/nadesiko-reversi-server
COPY --from=build /out/bin/gonako /app/bin/gonako
COPY --from=build /out/web /app/web
COPY --from=build /out/rules /app/rules

ENV GONAKO_BIN=/app/bin/gonako
ENV PORT=8080
EXPOSE 8080
USER demo

ENTRYPOINT ["/app/nadesiko-reversi-server", "-web", "/app/web", "-rules", "/app/rules"]
