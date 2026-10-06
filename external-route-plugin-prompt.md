# Submit a navigation route to PhMon

PhMon can draw a path that your plugin is already following. PhMon does not generate the path, call `start_script`, or stop your navigation. You submit the points when your route starts, when you replace it, and when you stop.

Look up PhMon at the moment you submit. Do not import it while your plugin is loading.

```python
import sys

def submit_route_to_phmon(route):
    phmon = sys.modules.get('PhMon')
    submit = getattr(phmon, 'submit_external_route', None) if phmon is not None else None
    if not callable(submit):
        return False
    return bool(submit(route))
```

`PhMon` is the module name phBot gives `Plugins/PhMon.py`. A missing module or a `False` return means the path is not shown. Do not retry in a loop, and do not call any PhMon navigation command.

## When to submit

Call `submit_external_route` once from a phBot callback:

- when your plugin starts a route
- when it replaces that route with a different one
- when it stops, finishes off the final point, or otherwise abandons the route

PhMon tracks the character and removes the part already walked. Do not send a new snapshot for every position update.

## Active route

```python
submit_route_to_phmon({
    'sequence': 1,
    'active': True,
    'source': {'region': 25000, 'x': 6410.0, 'y': 1080.0, 'z': 0.0},
    'lines': [
        'walk,6420,1080,0',
        'wait,500',
        'teleport,Hotan,Jangan',
        'walk,100,200,0',
    ],
})
```

- `sequence` is an integer starting at 1. Increase it for every new path. Repeating the same sequence does not reset the line already on the map.
- `active` is `True`.
- `source` is the character's region, x, y, and z when the route starts. A walk without its own region uses this to place the path. A walk that includes a region uses that region instead.
- `lines` are the points your plugin is actually following, in phBot game coordinates. Pass a list of lines or one script string. PhMon scans at most 8192 raw lines and keeps at most 4096 instructions. The encoded route must fit in 512 KiB. A single line longer than 256 characters is skipped. The kept route must include at least one `walk`.

These line forms are drawn:

```text
walk,6430.0,1090.0,-32.6
walk,25000,6420.0,1080.0,0
walk,-32767,-24200,10,0
wait,500
teleport,Hotan,Jangan
teleport,Ferry Ticket Seller Doji,Ferry Ticket Seller Tayun
teleport,Ferry (Doji),Ferry (Tayun)
```

`walk` may start with the phBot script region. That form is how ferry and cave scripts name the area of each point. Coordinates stay the in-game X/Y/Z. A walk with region 0, or with coordinates that are not finite or are outside 10,000,000, is skipped.

`wait` is a duration in milliseconds, from 0 through 999999. `teleport` is a break in the drawn line; the names may contain punctuation, and PhMon does not use them to teleport. Blank lines, comments (`#` or `//`), shop commands, and any other command are omitted. The submission returns `False` only when no `walk` remains, or when `sequence`, `active`, or `source` is invalid. The previous shown route stays as it was.

If you call `generate_script` yourself, keep that return value and submit those lines when your script actually starts. If you only call `move_to`, there is no path to submit.

## Stop

Send the same sequence with `active` set to `False` and no lines:

```python
submit_route_to_phmon({'sequence': 1, 'active': False})
```

Do this when navigation stops, including when the character stops somewhere other than the final walk point. PhMon then removes the line. That sequence cannot be shown again; the next route needs a higher sequence.

## What PhMon does with it

PhMon binds the snapshot to the current character session and draws the remaining walk segments. Each fresh position within 12 game units of the next segment drops the walked prefix and redraws from the character. A wait or teleport breaks the line until a new position shows that the character entered the next walk section.

Your plugin keeps owning the movement. PhMon will not call `stop_script` for this path.
