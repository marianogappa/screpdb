# StarCraft Dashboard Frontend

A React-based frontend for the StarCraft replay database dashboard system.

## Setup

1. Install dependencies:
```bash
npm install
```

2. Start the development server:
```bash
npm run dev
```

The frontend will run on `http://localhost:3000` and proxy API requests to `http://localhost:8000`.

## Building for Production

```bash
npm run build
```

The built files will be in the `dist` directory.

## Features

- View dashboards with widgets in a 2-column grid layout
- Create new widgets using natural language prompts
- Edit widgets manually (name, description, SQL query, HTML content)
- Delete widgets
- Manage dashboards (create, edit, delete, switch)
- Dark space-themed UI matching the StarCraft aesthetic
- Widget content supports HTML/CSS/JavaScript with D3.js for visualizations
- Each widget's SQL results are available as `sqlRowsForWidget{id}` JavaScript variable

## Development

The frontend uses:
- React 18
- Vite for build tooling
- D3.js (loaded via CDN) for chart visualizations

API endpoints are proxied through Vite's dev server to avoid CORS issues.

