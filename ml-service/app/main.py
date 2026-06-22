from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel
from starlette.datastructures import UploadFile


app = FastAPI(title="DentVision ML Service")


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


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/analyze", response_model=AnalyzeResponse)
async def analyze(request: Request) -> AnalyzeResponse:
    content_type = request.headers.get("content-type", "")
    has_image_input = False

    if content_type.startswith("application/json"):
        try:
            payload = await request.json()
        except ValueError as exc:
            raise HTTPException(status_code=400, detail="invalid json") from exc

        image_path = payload.get("image_path") if isinstance(payload, dict) else None
        has_image_input = isinstance(image_path, str) and image_path.strip() != ""

    elif content_type.startswith("multipart/form-data"):
        form = await request.form()
        image_path = form.get("image_path")
        image = form.get("image")

        has_image_input = isinstance(image_path, str) and image_path.strip() != ""

        if isinstance(image, UploadFile):
            await image.close()
            has_image_input = True

    if not has_image_input:
        raise HTTPException(status_code=400, detail="image_path or image file is required")

    return AnalyzeResponse(results=DEMO_RESULTS)
