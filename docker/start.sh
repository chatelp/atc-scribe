#!/bin/bash

# Co-ATC Docker Startup Script
# This script helps you get Co-ATC running quickly in Docker

set -e

echo "🛩️  Co-ATC Docker Setup"
echo "=================================================="

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if Docker Compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Navigate to docker directory
cd "$(dirname "$0")"
DOCKER_DIR=$(pwd)
PROJECT_DIR=$(dirname "$DOCKER_DIR")

echo "📁 Working in: $DOCKER_DIR"

# Check if assets directory exists
if [ ! -d "../assets" ]; then
    echo "❌ Assets directory not found at ../assets"
    echo "   This directory contains static data files (airlines, airports, runways)."
    echo "   Please ensure you're running this from the docker/ directory"
    echo "   and that the assets/ directory exists in the project root."
    exit 1
fi

# Check if prompts directory exists
if [ ! -d "../prompts" ]; then
    echo "❌ Prompts directory not found at ../prompts"
    echo "   This directory contains AI system prompts."
    echo "   Please ensure you're running this from the docker/ directory"
    echo "   and that the prompts/ directory exists in the project root."
    exit 1
fi

# Check if required files exist
REQUIRED_FILES=("../assets/airlines.json" "../assets/airports.json" "../assets/runways.json" "../prompts/atc_chat_prompt.txt")
for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "❌ Required file missing: $file"
        exit 1
    fi
done

echo "✅ Assets and prompts directories found"

# Check if config file exists
if [ ! -f "config.toml" ]; then
    echo "🔧 Creating configuration file from example..."
    cp ../configs/config.toml.example config.toml
    echo "✅ Configuration file created: config.toml"
    echo ""
    echo "⚠️  IMPORTANT: Please edit config.toml with your settings:"
    echo "   - Set your ADS-B source URL"
    echo "   - Configure your location coordinates"
    echo "   - Add OpenAI API keys (optional)"
    echo "   - Configure radio frequencies"
    echo ""
    echo "💡 After editing, run this script again to start the system."
    echo ""
    read -p "Press Enter to open the config file now or Ctrl+C to exit..."
    
    # Try to open the config file with common editors
    if command -v nano &> /dev/null; then
        nano config.toml
    elif command -v vim &> /dev/null; then
        vim config.toml
    elif command -v code &> /dev/null; then
        code config.toml
    else
        echo "Please edit config.toml with your preferred text editor."
        exit 0
    fi
fi

# Docker compose configuration is already correct
echo "🔧 Verifying docker-compose.yml configuration..."
if grep -q "./config.toml:/app/configs/config.toml:ro" docker-compose.yml; then
    echo "✅ Docker Compose configuration verified"
else
    echo "❌ Docker Compose configuration needs fixing"
    exit 1
fi

# Build and start the system
echo "🚀 Building and starting Co-ATC..."
docker-compose build
docker-compose up -d

echo ""
echo "🎉 Co-ATC is starting up!"
echo "================================"
echo "📱 Web Interface: http://localhost:8000"
echo "🔍 View logs: docker-compose logs -f co-atc"
echo "🛑 Stop system: docker-compose down"
echo ""

# Wait for the system to start
echo "⏳ Waiting for the system to start..."
sleep 5

# Check if the system is running
if docker-compose ps | grep -q "Up"; then
    echo "✅ Co-ATC is running successfully!"
    echo ""
    echo "🌐 Opening web interface..."
    
    # Try to open the web interface
    if command -v open &> /dev/null; then
        open http://localhost:8000
    elif command -v xdg-open &> /dev/null; then
        xdg-open http://localhost:8000
    else
        echo "Please open http://localhost:8000 in your browser"
    fi
else
    echo "❌ System failed to start. Checking logs..."
    docker-compose logs co-atc
fi

echo ""
echo "🛩️  Happy flying!" 