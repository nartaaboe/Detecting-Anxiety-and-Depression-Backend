from __future__ import annotations

from fastapi import FastAPI
from pydantic import BaseModel, Field


app = FastAPI(title="MoodInsight AI Mock", version="0.1.0")


class InferRequest(BaseModel):
    text: str = Field(min_length=1)
    model_version: str = "baseline"
    threshold: float = 0.5


class Explanation(BaseModel):
    key_phrases: list[str]
    top_sentences: list[str]


class InferResponse(BaseModel):
    label: str
    score: float
    confidence: float
    explanation: Explanation


@app.post("/infer", response_model=InferResponse)
def infer(req: InferRequest) -> InferResponse:
    text = req.text.strip()
    length_score = min(1.0, max(0.0, len(text) / 240.0))

    # Simple deterministic "risk" scoring mock.
    score = round(length_score, 4)
    confidence = 0.75 if req.model_version == "baseline" else 0.85

    if score < req.threshold:
        label = "low"
    elif score < 0.8:
        label = "medium"
    else:
        label = "high"

    words = [w for w in text.replace("\n", " ").split(" ") if w]
    key_phrases = words[: min(5, len(words))]
    top_sentences = [s.strip() for s in text.split(".") if s.strip()][:2] or [text[:160]]

    return InferResponse(
        label=label,
        score=score,
        confidence=confidence,
        explanation=Explanation(key_phrases=key_phrases, top_sentences=top_sentences),
    )

