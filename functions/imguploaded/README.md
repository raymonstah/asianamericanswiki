gcloud functions deploy imguploaded \
--gen2 \
--runtime=go121 \
--region=us-central1 \
--trigger-location=us \
--source=. \
--entry-point=ImgUploaded \
--trigger-event-filters="type=google.cloud.storage.object.v1.finalized" \
--trigger-event-filters="bucket=asianamericanswiki-images"
