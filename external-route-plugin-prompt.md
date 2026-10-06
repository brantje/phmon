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
- `source` is the character's region, x, y, and z when the route starts. Walk points do not carry a region, and this is what places the path, including the cave floor.
- `lines` are the points your plugin is actually following, in phBot game coordinates. At most 256 lines. Each line is at most 256 characters. The whole route must include at least one `walk`.

Only these three line forms are accepted:

```text
walk,6430.0,1090.0,-32.6
wait,500
teleport,Hotan,Jangan
```

`walk` coordinates are finite and within 10,000,000. `wait` is a duration in milliseconds, from 0 through 999999. `teleport` is a break in the drawn line; PhMon does not use the names to teleport. Any other command makes the whole submission return `False`, and the previous shown route stays as it was.

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
