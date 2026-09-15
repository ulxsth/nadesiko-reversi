#!/usr/bin/env bash
# 公開デモをCloud Runへデプロイする。
#
# GCPプロジェクトと課金は利用者が用意する。必要な環境変数が無ければ実行しない。
# 公開URLは初回デプロイまで決まらないので、デプロイ後にPUBLIC_ORIGINを書き戻す2段階で行う。
set -euo pipefail

PROJECT_ID="${PROJECT_ID:-}"
REGION="${REGION:-asia-northeast1}"
SERVICE="${SERVICE:-nadesiko-reversi-demo}"
REPOSITORY="${REPOSITORY:-nadesiko-reversi}"
IMAGE_TAG="${IMAGE_TAG:-$(date -u +%Y%m%d-%H%M%S)}"
CPU="${CPU:-1}"
MEMORY="${MEMORY:-512Mi}"
CONCURRENCY="${CONCURRENCY:-20}"
MAX_INSTANCES="${MAX_INSTANCES:-1}"
MIN_INSTANCES="${MIN_INSTANCES:-0}"
REQUEST_TIMEOUT="${REQUEST_TIMEOUT:-3600}"
DRY_RUN=0

usage() {
  cat <<'USAGE'
使い方: PROJECT_ID=<GCPプロジェクトID> scripts/deploy-cloud-run.sh [--dry-run]

環境変数:
  PROJECT_ID       必須。デプロイ先のGCPプロジェクトID。
  REGION           既定 asia-northeast1
  SERVICE          既定 nadesiko-reversi-demo
  REPOSITORY       既定 nadesiko-reversi（Artifact Registryのリポジトリ名）
  IMAGE_TAG        既定 UTCの日時
  CPU/MEMORY       既定 1 / 512Mi
  CONCURRENCY      既定 20
  MIN_INSTANCES    既定 0
  MAX_INSTANCES    既定 1
  REQUEST_TIMEOUT  既定 3600（秒。WebSocketの接続期限）

--dry-run を付けると、実行せずにコマンド列だけを表示する。
USAGE
}

for argument in "$@"; do
  case "$argument" in
    --dry-run) DRY_RUN=1 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "不明な引数です: $argument" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$PROJECT_ID" ]]; then
  echo "PROJECT_IDを指定してください。" >&2
  usage >&2
  exit 2
fi

if [[ "$DRY_RUN" -eq 0 ]] && ! command -v gcloud >/dev/null 2>&1; then
  echo "gcloudが見つかりません。Google Cloud CLIを入れてから実行してください。" >&2
  exit 2
fi

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPOSITORY}/${SERVICE}:${IMAGE_TAG}"

# run は副作用のあるコマンドをまとめて表示し、--dry-runでは実行しない。
run() {
  printf '+ %s\n' "$*"
  if [[ "$DRY_RUN" -eq 0 ]]; then
    "$@"
  fi
}

echo "== 1. APIを有効化する =="
run gcloud services enable run.googleapis.com artifactregistry.googleapis.com cloudbuild.googleapis.com \
  --project "$PROJECT_ID"

echo "== 2. Artifact Registryのリポジトリを用意する =="
if [[ "$DRY_RUN" -eq 1 ]]; then
  printf '+ %s\n' "gcloud artifacts repositories create $REPOSITORY --repository-format=docker --location=$REGION --project=$PROJECT_ID （既にあれば省略）"
elif ! gcloud artifacts repositories describe "$REPOSITORY" \
  --location "$REGION" --project "$PROJECT_ID" >/dev/null 2>&1; then
  run gcloud artifacts repositories create "$REPOSITORY" \
    --repository-format=docker \
    --location "$REGION" \
    --project "$PROJECT_ID" \
    --description "なでしこリバーシ公開デモ"
else
  echo "リポジトリ ${REPOSITORY} は既にあります。"
fi

echo "== 3. イメージをbuildする =="
run gcloud builds submit "$ROOT_DIR" \
  --tag "$IMAGE" \
  --project "$PROJECT_ID" \
  --region "$REGION"

echo "== 4. Cloud Runへデプロイする =="
run gcloud run deploy "$SERVICE" \
  --image "$IMAGE" \
  --project "$PROJECT_ID" \
  --region "$REGION" \
  --platform managed \
  --allow-unauthenticated \
  --cpu "$CPU" \
  --memory "$MEMORY" \
  --concurrency "$CONCURRENCY" \
  --min-instances "$MIN_INSTANCES" \
  --max-instances "$MAX_INSTANCES" \
  --timeout "$REQUEST_TIMEOUT" \
  --no-use-http2 \
  --port 8080

echo "== 5. 公開URLをPUBLIC_ORIGINへ書き戻す =="
if [[ "$DRY_RUN" -eq 1 ]]; then
  printf '+ %s\n' "gcloud run services describe $SERVICE --region $REGION --project $PROJECT_ID --format value(status.url)"
  printf '+ %s\n' "gcloud run services update $SERVICE --update-env-vars PUBLIC_ORIGIN=<発行されたURL> --region $REGION --project $PROJECT_ID"
  echo "（--dry-runのためここで終了します）"
  exit 0
fi

SERVICE_URL="$(gcloud run services describe "$SERVICE" \
  --region "$REGION" --project "$PROJECT_ID" --format 'value(status.url)')"
if [[ -z "$SERVICE_URL" ]]; then
  echo "公開URLを取得できませんでした。" >&2
  exit 1
fi

CURRENT_ORIGIN="$(gcloud run services describe "$SERVICE" \
  --region "$REGION" --project "$PROJECT_ID" \
  --format 'value(spec.template.spec.containers[0].env.filter("name:PUBLIC_ORIGIN").extract("value").flatten())')"

if [[ "$CURRENT_ORIGIN" == "$SERVICE_URL" ]]; then
  echo "PUBLIC_ORIGINは既に ${SERVICE_URL} です。"
else
  run gcloud run services update "$SERVICE" \
    --update-env-vars "PUBLIC_ORIGIN=${SERVICE_URL}" \
    --region "$REGION" \
    --project "$PROJECT_ID"
fi

echo ""
echo "公開URL: ${SERVICE_URL}"
echo "確認: curl -fsS ${SERVICE_URL}/healthz"
echo "費用の確認とBilling予算アラートは docs/deployment-cloud-run.md を参照してください。"
