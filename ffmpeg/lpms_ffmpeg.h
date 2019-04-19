#include <libavutil/hwcontext.h>
#include <libavutil/rational.h>

typedef struct {
  char *fname;
  char *vencoder;
  char *vfilters;
  int w, h, bitrate;
  AVRational fps;
} output_params;

typedef struct {
  char *fname;

  // Optional hardware acceleration
  enum AVHWDeviceType hw_type;
} input_params;

void lpms_init();
void lpms_deinit();
int  lpms_rtmp2hls(char *listen, char *outf, char *ts_tmpl, char *seg_time, char *seg_start);
int  lpms_transcode(input_params *inp, output_params *params, int nb_outputs);
int  lpms_length(char *inp, int ts_max, int packet_max);
