"""What the sidecar computes on, as /health declares it in `backend`."""
from __future__ import annotations


def backend_name() -> str:
    """The GPU's name when mlx.core gives it ("Apple M2 Pro"), else "mlx".

    MLX has exposed the device's description under two names over its versions
    (mx.device_info, mx.metal.device_info), and not every version names the
    device. Anything missing or failing falls back to "mlx": this is a label for
    the sign-in page, never a reason not to start.
    """
    try:
        import mlx.core as mx
    except Exception:
        return "mlx"
    for owner in (mx, getattr(mx, "metal", None)):
        fn = getattr(owner, "device_info", None)
        if fn is None:
            continue
        try:
            info = fn()
        except Exception:
            continue
        name = str((info or {}).get("device_name") or "").strip()
        if name:
            return name[:48]
    return "mlx"
