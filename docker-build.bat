@echo off
REM Vessel Telemetry API Docker Build Script for Windows

setlocal enabledelayedexpansion

REM Configuration
set IMAGE_NAME=vessel-telemetry-api
set TAG=%1
if "%TAG%"=="" set TAG=latest
set FULL_IMAGE_NAME=%IMAGE_NAME%:%TAG%

echo.
echo 🚢 Building Vessel Telemetry API Docker Image
echo Image: %FULL_IMAGE_NAME%
echo.

REM Create data directory if it doesn't exist
if not exist "data" (
    echo 📁 Creating data directory...
    mkdir data
)

REM Build the Docker image
echo 🔨 Building Docker image...
docker build -t %FULL_IMAGE_NAME% .

if %ERRORLEVEL% equ 0 (
    echo.
    echo ✅ Docker image built successfully!
    echo.
    
    REM Show image info
    echo 📊 Image Information:
    docker images %IMAGE_NAME%:%TAG%
    echo.
    
    REM Ask if user wants to run the container
    set /p REPLY="🚀 Do you want to start the container with docker-compose? (y/n): "
    
    if /i "!REPLY!"=="y" (
        echo.
        echo 🚀 Starting container with docker-compose...
        docker-compose up -d
        
        echo.
        echo ✅ Container started successfully!
        echo 📍 API available at: http://localhost:8080
        echo 🏥 Health check: http://localhost:8080/healthz
        echo 📊 Dashboard: http://localhost:8080/dashboard.html
        echo.
        echo 📋 Useful commands:
        echo   docker-compose logs -f    # View logs
        echo   docker-compose stop       # Stop container
        echo   docker-compose down       # Stop and remove container
    )
) else (
    echo.
    echo ❌ Docker build failed!
    exit /b 1
)

pause