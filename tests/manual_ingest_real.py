import asyncio
import os
import sys
import random
from reportlab.pdfgen import canvas
from PIL import Image, ImageDraw
from dotenv import load_dotenv

# Path setup
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'ai')))

# Load Env
root_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
load_dotenv(os.path.join(root_dir, '.env'))

from document_ingester import DocumentIngester
from llm_router import LLMRouter

async def manual_test():
    print("--- Manual Ingestion Test (Real PDF + Real API) ---")
    
    pdf_path = "temp_vision_test.pdf"
    img_path = "temp_chart.png"
    
    try:
        # 1. Generate High-Complexity Image (Fake Chart)
        print("Generating complex image...")
        img = Image.new('RGB', (400, 300), color='white')
        draw = ImageDraw.Draw(img)
        # Draw random lines to ensure high entropy/edge density
        for _ in range(1000):
            x1, y1 = random.randint(0, 400), random.randint(0, 300)
            x2, y2 = random.randint(0, 400), random.randint(0, 300)
            draw.line([(x1, y1), (x2, y2)], fill='black', width=1)
        img.save(img_path)
        
        # 2. Generate PDF with embedded image
        print("Generating PDF...")
        c = canvas.Canvas(pdf_path)
        c.drawString(100, 750, "Project Alpha Q3 Performance Report")
        c.drawString(100, 730, "The following chart shows the trajectory:")
        c.drawImage(img_path, 100, 400, width=400, height=300)
        c.save()
        
        # 3. Initialize Real Components
        router = LLMRouter()
        ingester = DocumentIngester(router)
        
        # 4. Mock Extraction (Bypass fitz) & Ingest
        print("Ingesting (Simulating PDF Extraction)...")
        
        # Load the generated image and encode to base64
        import base64
        from io import BytesIO
        from document_ingester import ExtractedImage
        
        with open(img_path, "rb") as f:
            img_bytes = f.read()
            img_b64 = base64.b64encode(img_bytes).decode()
            
        # Create simulation data
        dummy_text = "This document contains a complex performance chart shown below."
        dummy_pages = [(1, dummy_text)]
        dummy_images = [
            ExtractedImage(
                image_base64=img_b64,
                page_number=1,
                image_index=0
            )
        ]
        
        # Call processing directly (Public API would be ingest_pdf, but we lack pymupdf)
        # We access the internal method to test the PIPELINE logic + API
        print("calling _process_document with Real NVIDIA API...")
        result = await ingester._process_document(dummy_pages, dummy_images)
        
        # 5. Report
        print("\n--- Ingestion Results ---")
        print(f"Chunks: {len(result.chunks)}")
        print(f"Entities: {len(result.entities)}")
        
        # Check for Vision-sourced entities
        vision_entities = [e for e in result.entities if e.source == "vision" or e.confidence == 0.8] 
        # Note: document_ingester sets source="vision" ? 
        # Checking implementation: logic calls _extract_with_vision.
        # I need to check what source it sets. Assuming "vision" or similar if logic is separate.
        # Actually _extract_with_vision returns entities.
        
        if len(result.entities) > 0:
            print("Entities found:")
            for e in result.entities:
                print(f" - {e.name} ({e.entity_type}): {e.description}")
        else:
            print("WARNING: No entities extracted.")
            
    except Exception as e:
        print(f"ERROR: {e}")
        import traceback
        traceback.print_exc()
    finally:
        # Cleanup
        if os.path.exists(pdf_path):
            os.remove(pdf_path)
        if os.path.exists(img_path):
            os.remove(img_path)

if __name__ == "__main__":
    asyncio.run(manual_test())
