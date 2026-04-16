import os
from app import create_app
from flask import send_from_directory
app = create_app()
DIST = os.environ.get('LLSTACK_DIST_DIR', '/opt/llstack/web/dist')

@app.route('/', defaults={'path': ''})
@app.route('/<path:path>')
def serve(path):
    # Never intercept API or WebSocket routes (handled by blueprints)
    if path.startswith(('api/', 'ws/')):
        from flask import abort
        abort(404)
    f = os.path.join(DIST, path)
    if path and os.path.isfile(f):
        return send_from_directory(DIST, path)
    return send_from_directory(DIST, 'index.html')
