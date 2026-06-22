import os
from io import BytesIO
from pathlib import Path
from typing import Any

from fastapi import FastAPI, HTTPException, Request
from PIL import Image, UnidentifiedImageError
from pydantic import BaseModel
from starlette.datastructures import UploadFile

try:
    from ultralytics import YOLO
except Exception as exc:  # pragma: no cover - covered by container smoke checks.
    YOLO = None  # type: ignore[assignment]
    YOLO_IMPORT_ERROR = exc
else:
    YOLO_IMPORT_ERROR = None


MODEL_LABELS = {
    0: "impacted",
    1: "caries",
    2: "periapical_lesion",
    3: "deep_caries",
}


def env_bool(key: str, default: bool) -> bool:
    value = os.getenv(key)
    if value is None:
        return default

    return value.strip().lower() in {"1", "true", "yes", "on"}


MODEL_PATH = os.getenv("MODEL_PATH", "/app/models/dentex_yolov8n.pt")
MODEL_CONFIDENCE = float(os.getenv("MODEL_CONFIDENCE", "0.25"))
MODEL_IMAGE_SIZE = int(os.getenv("MODEL_IMAGE_SIZE", "640"))
ML_DEMO_MODE = env_bool("ML_DEMO_MODE", False)

app = FastAPI(title="DentVision ML Service")
model: Any | None = None
model_error = ""


class AnalysisRegion(BaseModel):
    label: str
    confidence: float
    x: int
    y: int
    width: int
    height: int


class AnalyzeResponse(BaseModel):
    results: list[AnalysisRegion]


DEMO_RESULTS = [
    AnalysisRegion(
        label="suspicious_area",
        confidence=0.87,
        x=120,
        y=90,
        width=180,
        height=120,
    ),
    AnalysisRegion(
        label="suspicious_area",
        confidence=0.76,
        x=360,
        y=160,
        width=140,
        height=110,
    ),
]


@app.on_event("startup")
def load_model() -> None:
    global model, model_error

    model_path = Path(MODEL_PATH)
    if YOLO is None:
        model = None
        model_error = f"ultralytics import failed: {YOLO_IMPORT_ERROR}"
        return

    if not model_path.exists():
        model = None
        model_error = f"model file not found: {model_path}"
        return

    try:
        model = YOLO(str(model_path))
        model_error = ""
    except Exception as exc:
        model = None
        model_error = f"model load failed: {exc}"


@app.get("/health")
def health() -> dict[str, bool | str]:
    model_loaded = model is not None
    mode = "model" if model_loaded or not ML_DEMO_MODE else "demo"

    response: dict[str, bool | str] = {
        "status": "ok",
        "model_loaded": model_loaded,
        "mode": mode,
    }

    if not model_loaded and model_error:
        response["model_error"] = model_error

    return response


@app.post("/analyze", response_model=AnalyzeResponse)
async def analyze(request: Request) -> AnalyzeResponse:
    source = await read_analysis_source(request)

    if model is None:
        if ML_DEMO_MODE:
            return AnalyzeResponse(results=DEMO_RESULTS)

        raise HTTPException(
            status_code=503,
            detail=model_error or "model is not loaded",
        )

    try:
        predictions = model.predict(
            source=source,
            conf=MODEL_CONFIDENCE,
            imgsz=MODEL_IMAGE_SIZE,
            verbose=False,
        )
    except Exception as exc:
        raise HTTPException(status_code=500, detail=f"model inference failed: {exc}") from exc

    return AnalyzeResponse(results=parse_yolo_results(predictions))


async def read_analysis_source(request: Request) -> str | Image.Image:
    content_type = request.headers.get("content-type", "")

    if content_type.startswith("application/json"):
        try:
            payload = await request.json()
        except ValueError as exc:
            raise HTTPException(status_code=400, detail="invalid json") from exc

        image_path = payload.get("image_path") if isinstance(payload, dict) else None
        if isinstance(image_path, str) and image_path.strip():
            return validate_image_path(image_path)

    elif content_type.startswith("multipart/form-data"):
        form = await request.form()
        image_path = form.get("image_path")
        image = form.get("image")

        if isinstance(image_path, str) and image_path.strip():
            if isinstance(image, UploadFile):
                await image.close()
            return validate_image_path(image_path)

        if isinstance(image, UploadFile):
            try:
                data = await image.read()
            finally:
                await image.close()

            if not data:
                raise HTTPException(status_code=400, detail="empty image file")

            try:
                return Image.open(BytesIO(data)).convert("RGB")
            except UnidentifiedImageError as exc:
                raise HTTPException(status_code=400, detail="invalid image file") from exc

    raise HTTPException(status_code=400, detail="image_path or image file is required")


def validate_image_path(image_path: str) -> str:
    path = Path(image_path)
    if not path.exists():
        raise HTTPException(status_code=400, detail=f"image file not found: {image_path}")

    if not path.is_file():
        raise HTTPException(status_code=400, detail=f"image path is not a file: {image_path}")

    return str(path)


def parse_yolo_results(predictions: Any) -> list[AnalysisRegion]:
    regions: list[AnalysisRegion] = []

    for prediction in predictions:
        boxes = getattr(prediction, "boxes", None)
        if boxes is None:
            continue

        for box in boxes:
            xyxy = box.xyxy[0].detach().cpu().tolist()
            confidence = float(box.conf[0].detach().cpu().item())
            class_id = int(box.cls[0].detach().cpu().item())
            x1, y1, x2, y2 = xyxy

            regions.append(
                AnalysisRegion(
                    label=MODEL_LABELS.get(class_id, f"class_{class_id}"),
                    confidence=round(confidence, 4),
                    x=round(x1),
                    y=round(y1),
                    width=max(1, round(x2 - x1)),
                    height=max(1, round(y2 - y1)),
                )
            )

    return regions
