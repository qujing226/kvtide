#!/usr/bin/env python3
import argparse
import json

from transformers import AutoTokenizer


DEFAULT_TEXTS = [
    "hello world",
    "Hello, world!",
    "I'm 18.",
    "你好，世界！",
    "e\u0301",
    "<|endoftext|>",
    "hello<|endoftext|>world",
]


def main() -> None:
    parser = argparse.ArgumentParser(description="Generate Qwen tokenizer golden cases.")
    parser.add_argument("--model", required=True, help="HuggingFace model id or local model directory")
    parser.add_argument("--output", required=True, help="Output JSON path")
    parser.add_argument("--text", action="append", default=[], help="Additional text case")
    args = parser.parse_args()

    tok = AutoTokenizer.from_pretrained(args.model, trust_remote_code=True)
    texts = DEFAULT_TEXTS + args.text

    cases = []
    for i, text in enumerate(texts):
        ids = tok.encode(text, add_special_tokens=False)
        decoded = tok.decode(ids)
        cases.append({
            "name": f"case_{i}",
            "text": text,
            "ids": ids,
            "decoded": decoded,
        })

    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(cases, f, ensure_ascii=False, indent=2)
        f.write("\n")


if __name__ == "__main__":
    main()
