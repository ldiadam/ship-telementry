#!/bin/bash

# Vessel Telemetry API Docker Build Script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
IMAGE_NAME="vessel-telemetry-api"
TAG=${1:-latest}
FULL_IMAGE_NAME="$IMAGE_NAME:$TAG"

echo -e "${BLUE}🚢 Building Vessel Telemetry API Docker Image${NC}"
echo -e "${YELLOW}Image: $FULL_IMAGE_NAME${NC}"
echo ""

# Create data directory if it doesn't exist
if [ ! -d "data" ]; then
    echo -e "${YELLOW}📁 Creating data directory...${NC}"
    mkdir -p data
fi

# Build the Docker image
echo -e "${BLUE}🔨 Building Docker image...${NC}"
docker build -t $FULL_IMAGE_NAME .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Docker image built successfully!${NC}"
    echo ""
    
    # Show image info
    echo -e "${BLUE}📊 Image Information:${NC}"
    docker images $IMAGE_NAME:$TAG
    echo ""
    
    # Ask if user wants to run the container
    read -p "🚀 Do you want to start the container with docker-compose? (y/n): " -n 1 -r
    echo ""
    
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}🚀 Starting container with docker-compose...${NC}"
        docker-compose up -d
        
        echo ""
        echo -e "${GREEN}✅ Container started successfully!${NC}"
        echo -e "${YELLOW}📍 API available at: http://localhost:31180${NC}"
        echo -e "${YELLOW}🏥 Health check: http://localhost:31180/healthz${NC}"
        echo -e "${YELLOW}📊 Dashboard: http://localhost:31180/dashboard.html${NC}"
        echo ""
        echo -e "${BLUE}📋 Useful commands:${NC}"
        echo "  docker-compose logs -f    # View logs"
        echo "  docker-compose stop       # Stop container"
        echo "  docker-compose down       # Stop and remove container"
    fi
else
    echo -e "${RED}❌ Docker build failed!${NC}"
    exit 1
fi