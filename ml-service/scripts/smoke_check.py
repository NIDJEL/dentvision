import json
import sys
import urllib.error
import urllib.request


def request_json(url: str, payload: dict[str, str] | None = None) -> dict:
    data = None
    headers = {}

    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"

    request = urllib.request.Request(url, data=data, headers=headers)

    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8")
        raise RuntimeError(f"{url} failed with status {exc.code}: {body}") from exc


def main() -> int:
    base_url = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8001"
    image_path = sys.argv[2] if len(sys.argv) > 2 else ""

    health = request_json(f"{base_url.rstrip('/')}/health")
    print(json.dumps({"health": health}, ensure_ascii=False))

    if not image_path:
        print("Pass an image path as the second argument to smoke-test /analyze.")
        return 0

    analysis = request_json(
        f"{base_url.rstrip('/')}/analyze",
        {"image_path": image_path},
    )

    if "results" not in analysis or not isinstance(analysis["results"], list):
        raise RuntimeError("/analyze response does not contain a results array")

    print(json.dumps({"result_count": len(analysis["results"])}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
