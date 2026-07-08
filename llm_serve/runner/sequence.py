from dataclasses import dataclass, field

@dataclass
class SequenceState:
    request_id: str
    token_ids: list[int] = field(default_factory=list)
    generated_tokens: int = 0
    done: bool = False