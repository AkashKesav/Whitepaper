import unittest
import sys
import os
import asyncio
from unittest.mock import MagicMock, AsyncMock, patch

# Add ai directory to path
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..', 'ai')))

# Mock fitz before importing document_ingester
sys.modules['fitz'] = MagicMock()
from document_ingester import DocumentIngester
from llm_router import LLMRouter

class TestIngestionPipeline(unittest.TestCase):
    def setUp(self):
        self.router = MagicMock(spec=LLMRouter)
        # Mock async methods
        self.router.generate = AsyncMock(return_value='{"entities": [{"name": "TestEntity", "type": "Concept"}]}')
        self.router.generate_vision = AsyncMock(return_value="Analysis of the chart shows growth.")
        self.router.extract_json = AsyncMock(return_value={"entities": [{"name": "TestEntity", "type": "Concept"}]})
        
        self.ingester = DocumentIngester(self.router)

    @patch('document_ingester.fitz')
    def test_pdf_ingestion_flow(self, mock_fitz):
        print("\n--- Testing PDF Ingestion Flow (Mocked) ---")
        
        # 1. Setup Mock PDF Document
        mock_doc = MagicMock()
        mock_page = MagicMock()
        
        # Mock Text
        mock_page.get_text.return_value = "This is a test document with a chart."
        
        # Mock Images (get_images returns list of tuples, item[0] is xref)
        mock_page.get_images.return_value = [(123, 0, 0, 0, 0, 0, 0, 'img1', 0)]
        
        # Mock Image Extraction (extract_image returns dict)
        # Make image large enough to pass complexity/size filters (>10KB approx)
        mock_doc.extract_image.return_value = {
            "ext": "png",
            "image": b"a" * 50000  # 50KB dummy image
        }
        
        # Page iteration
        mock_doc.__iter__.return_value = [mock_page]
        mock_doc.page_count = 1
        
        mock_fitz.open.return_value = mock_doc
        
        # 2. Run Ingestion with patched complexity to ensure Vision is called
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        with patch.object(DocumentIngester, '_calculate_complexity', return_value=6.0):
             result = loop.run_until_complete(
                self.ingester.ingest_pdf("dummy_path.pdf")
             )
        loop.close()
        
        # 3. Verify Results
        print(f"Result Type: {type(result)}")
        # Handle Pydantic model (use attributes)
        
        # Check Vision API was called
        # Verify Text Extraction
        self.assertEqual(len(result.chunks), 1)
        print("Text extraction verified.")
        
        # Verify Entity Extraction call
        self.router.extract_json.assert_called()
        print("LLM Entity Extraction verified.")
        
        # Verify Vision was called (forced by complexity=1.0)
        # We need to ensure logic flow reached generate_vision
        # If Logic: process document -> if image complex -> generate_vision
        # We mocked complexity=1.0. 
        # Check generate_vision called
        self.router.generate_vision.assert_called()
        print("Vision API call verified.")

        print("Success: Pipeline ran end-to-end.")

if __name__ == '__main__':
    unittest.main()
