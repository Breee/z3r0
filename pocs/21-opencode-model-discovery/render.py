# Runs inside the sandbox. Reads the proxy's model list from /tmp/poc21/models.json
# (fetched with curl, the program the sandbox's rule lets reach the proxy) and
#   stats            prints how many models it has and how many carry token limits
#   nolimits         prints the models that carry no token limits
#   write DIR ID...  writes DIR/opencode.json with a custom provider "poc21" holding these models,
#                    each with the limits the proxy reports
import json, os, sys

BASE = "http://host.openshell.internal:18096/v1"
models = {m["id"]: m for m in json.load(open("/tmp/poc21/models.json"))["data"]}

if sys.argv[1] == "stats":
    lim = [m for m in models.values() if m.get("max_input_tokens") and m.get("max_output_tokens")]
    owners = sorted({m.get("owned_by", "") for m in models.values()})
    modes = {}
    for m in models.values():
        modes[m.get("mode") or "none"] = modes.get(m.get("mode") or "none", 0) + 1
    big = max(lim, key=lambda m: m["max_input_tokens"]) if lim else None
    print("models=%d with_limits=%d owned_by=%s modes=%s largest=%s" % (
        len(models), len(lim), ",".join(owners),
        ",".join("%s:%d" % kv for kv in sorted(modes.items())),
        "%s:%d/%d" % (big["id"], big["max_input_tokens"], big["max_output_tokens"]) if big else "none"))
    sys.exit(0)

if sys.argv[1] == "nolimits":
    print(" ".join(sorted(i for i, m in models.items() if not (m.get("max_input_tokens") and m.get("max_output_tokens")))) or "none")
    sys.exit(0)

out = {}
for i in sys.argv[3:]:
    m = models[i]
    e = {"name": i}
    if m.get("max_input_tokens") and m.get("max_output_tokens"):
        e["limit"] = {"context": m["max_input_tokens"], "output": m["max_output_tokens"]}
    if "claude" in i:
        e["compatibility"] = {"cacheFormat": "anthropic"}
    out[i] = e
cfg = {
    "$schema": "https://opencode.ai/config.json",
    "providers": {"poc21": {
        "name": "PoC 21",
        "env": ["LITELLM_API_KEY"],
        "package": "@opencode/ai/providers/openai-compatible",
        "settings": {"baseURL": BASE, "apiKey": "{env:LITELLM_API_KEY}"},
        "models": out,
    }},
}
os.makedirs(sys.argv[2], exist_ok=True)
json.dump(cfg, open(os.path.join(sys.argv[2], "opencode.json"), "w"), indent=2)
print("wrote %d models" % len(out))
