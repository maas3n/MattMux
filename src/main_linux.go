//go:build linux && !cli

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

type linuxSettings struct { OutputDir string `json:"output_dir"`; PreserveChapters bool `json:"preserve_chapters"` }
type linuxGUI struct {
	window fyne.Window
	sourceEntry, outputEntry *widget.Entry
	titleSelect *widget.Select
	preserve *widget.Check
	progress *widget.ProgressBar
	status *widget.Label
	scanBtn, metaBtn, remuxBtn, cancelBtn, dvdBtn, isoBtn, outputBtn *widget.Button
	mu sync.Mutex
	busy bool
	cancel context.CancelFunc
	titles []titleInfo
	titlesSource string
}

func main(){a:=app.NewWithID("io.github.maas3n.mattmux");w:=a.NewWindow(fmt.Sprintf("MattMux %s",appVersion));g:=&linuxGUI{window:w};g.build();w.Resize(fyne.NewSize(840,620));w.CenterOnScreen();w.Show();go g.checkInstalledTools();a.Run()}

func (g *linuxGUI) build(){s:=loadLinuxSettings();if s.OutputDir==""{s.OutputDir=defaultLinuxOutputDir()};g.sourceEntry=widget.NewEntry();g.sourceEntry.SetPlaceHolder("/path/to/DVD, VIDEO_TS, or disc.iso");g.outputEntry=widget.NewEntry();g.outputEntry.SetText(s.OutputDir);g.titleSelect=widget.NewSelect(nil,nil);g.titleSelect.PlaceHolder="Scan titles first";g.preserve=widget.NewCheck("Preserve chapters in the output MKV",func(bool){g.saveSettings()});g.preserve.SetChecked(s.PreserveChapters);g.progress=widget.NewProgressBar();g.status=widget.NewLabel("Checking installed FFmpeg / FFprobe / MediaInfo…");g.status.Wrapping=fyne.TextWrapWord
	g.dvdBtn=widget.NewButton("DVD Folder…",g.chooseDVDFolder);g.isoBtn=widget.NewButton("ISO File…",g.chooseISO);g.outputBtn=widget.NewButton("Browse…",g.chooseOutput);g.scanBtn=widget.NewButton("Scan Titles",func(){g.startAsync("Scanning DVD titles…",g.scan)});g.metaBtn=widget.NewButton("Show Metadata",func(){g.startAsync("Reading title metadata…",g.showMetadata)});g.remuxBtn=widget.NewButton("Start Remux",func(){g.startAsync("Preparing remux…",g.remux)});g.remuxBtn.Importance=widget.HighImportance;g.cancelBtn=widget.NewButton("Cancel",g.cancelCurrent);g.cancelBtn.Disable();aboutBtn:=widget.NewButton("About",g.showAbout)
	g.sourceEntry.OnChanged=func(string){g.invalidateTitles()};g.outputEntry.OnChanged=func(string){g.saveSettings()}
	header:=container.NewVBox(widget.NewLabelWithStyle("MattMux",fyne.TextAlignLeading,fyne.TextStyle{Bold:true}),widget.NewLabel("Lossless DVD title remuxing to Matroska — video, audio, subtitles, chapters and metadata."));sourceRow:=container.NewBorder(nil,nil,nil,container.NewHBox(g.dvdBtn,g.isoBtn),g.sourceEntry);outputRow:=container.NewBorder(nil,nil,nil,g.outputBtn,g.outputEntry);titleRow:=container.NewBorder(nil,nil,nil,container.NewHBox(g.scanBtn,g.metaBtn,aboutBtn),g.titleSelect);actions:=container.NewHBox(layout.NewSpacer(),g.remuxBtn,g.cancelBtn)
	g.window.SetContent(container.NewPadded(container.NewVBox(header,widget.NewSeparator(),widget.NewLabelWithStyle("Source",fyne.TextAlignLeading,fyne.TextStyle{Bold:true}),sourceRow,widget.NewLabel("Choose a DVD folder / VIDEO_TS structure or an ISO image."),widget.NewSeparator(),widget.NewLabelWithStyle("Destination",fyne.TextAlignLeading,fyne.TextStyle{Bold:true}),outputRow,widget.NewSeparator(),widget.NewLabelWithStyle("DVD Title",fyne.TextAlignLeading,fyne.TextStyle{Bold:true}),titleRow,g.preserve,widget.NewSeparator(),g.progress,g.status,layout.NewSpacer(),actions)))}

func (g *linuxGUI) chooseDVDFolder(){d:=dialog.NewFolderOpen(func(uri fyne.ListableURI,err error){if err!=nil{g.showError(err);return};if uri!=nil{g.sourceEntry.SetText(uri.Path())}},g.window);d.Show()}
func (g *linuxGUI) chooseISO(){d:=dialog.NewFileOpen(func(r fyne.URIReadCloser,err error){if err!=nil{g.showError(err);return};if r==nil{return};defer r.Close();g.sourceEntry.SetText(r.URI().Path())},g.window);d.SetFilter(storage.NewExtensionFileFilter([]string{".iso",".ISO"}));d.Show()}
func (g *linuxGUI) chooseOutput(){d:=dialog.NewFolderOpen(func(uri fyne.ListableURI,err error){if err!=nil{g.showError(err);return};if uri!=nil{g.outputEntry.SetText(uri.Path())}},g.window);d.Show()}
func (g *linuxGUI) checkInstalledTools(){ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel();line:="System FFmpeg is missing/incompatible; MattMux will prepare its verified fallback when needed.";if tools,err:=inspectSystemTools(ctx);err==nil{line=fmt.Sprintf("Ready — using system FFmpeg (%s).",tools.ffmpeg)};fyne.Do(func(){g.status.SetText(line)})}
func (g *linuxGUI) startAsync(label string,fn func(context.Context)error){g.mu.Lock();if g.busy{g.mu.Unlock();return};ctx,cancel:=context.WithCancel(context.Background());g.busy,g.cancel=true,cancel;g.mu.Unlock();g.setBusy(true);g.setProgress(0,label);go func(){err:=fn(ctx);g.mu.Lock();g.busy,g.cancel=false,nil;g.mu.Unlock();fyne.Do(func(){g.setBusy(false);if errors.Is(err,context.Canceled){g.progress.SetValue(0);g.status.SetText("Operation cancelled.");return};if err!=nil{g.status.SetText("Failed: "+firstLine(err.Error()));dialog.ShowError(err,g.window)}})}()}
func (g *linuxGUI) setBusy(b bool){controls:=[]interface{Disable();Enable()}{g.sourceEntry,g.outputEntry,g.titleSelect,g.preserve,g.scanBtn,g.metaBtn,g.remuxBtn,g.dvdBtn,g.isoBtn,g.outputBtn};for _,c:=range controls{if b{c.Disable()}else{c.Enable()}};if b{g.cancelBtn.Enable()}else{g.cancelBtn.Disable()}}
func (g *linuxGUI) cancelCurrent(){g.mu.Lock();c:=g.cancel;g.mu.Unlock();if c!=nil{g.status.SetText("Cancelling…");c()}}

func (g *linuxGUI) scan(ctx context.Context)error{src,err:=normalizeSource(g.sourceEntry.Text);if err!=nil{return err};tools,err:=ensureTools(ctx,false,g.progressCallback());if err!=nil{return err};titles,err:=scanTitles(ctx,src,tools,g.progressCallback());if err!=nil{return err};best,_:=longestTitle(titles);opts:=make([]string,0,len(titles));selected:="";for _,t:=range titles{label:=fmt.Sprintf("Title %d — %s",t.Number,formatDuration(t.Duration));opts=append(opts,label);if t.Number==best.Number{selected=label}};g.mu.Lock();g.titles=append([]titleInfo(nil),titles...);g.titlesSource=src;g.mu.Unlock();fyne.Do(func(){g.titleSelect.Options=opts;g.titleSelect.Refresh();g.titleSelect.SetSelected(selected);g.progress.SetValue(1);g.status.SetText(fmt.Sprintf("Found %d title(s). Selected title %d (%s) as the longest.",len(titles),best.Number,formatDuration(best.Duration)))});return nil}
func parseTitleLabel(label string) int { fields:=strings.Fields(strings.TrimSpace(label)); if len(fields)<2 || fields[0]!="Title" { return 0 }; n,err:=strconv.Atoi(fields[1]); if err!=nil || n<1 { return 0 }; return n }
func (g *linuxGUI) selectedTitle()(string,titleInfo,error){src,err:=normalizeSource(g.sourceEntry.Text);if err!=nil{return "",titleInfo{},err};n:=parseTitleLabel(g.titleSelect.Selected);if n<1{return "",titleInfo{},errors.New("scan the DVD and choose a title first")};g.mu.Lock();defer g.mu.Unlock();if filepath.Clean(src)!=filepath.Clean(g.titlesSource){return "",titleInfo{},errors.New("source changed after the last title scan; scan again")};for _,t:=range g.titles{if t.Number==n{return src,t,nil}};return "",titleInfo{},errors.New("selected title is no longer available; scan again")}
func (g *linuxGUI) showMetadata(ctx context.Context)error{src,title,err:=g.selectedTitle();if err!=nil{return err};tools,err:=ensureTools(ctx,true,g.progressCallback());if err!=nil{return err};text,err:=metadataText(ctx,src,title,tools,g.preserve.Checked);if err!=nil{return err};fyne.Do(func(){entry:=widget.NewMultiLineEntry();entry.SetText(text);entry.Disable();w:=fyne.CurrentApp().NewWindow(fmt.Sprintf("MattMux — Title %d metadata",title.Number));w.SetContent(entry);w.Resize(fyne.NewSize(760,600));w.Show();g.progress.SetValue(1);g.status.SetText(fmt.Sprintf("Metadata loaded for title %d.",title.Number))});return nil}
func (g *linuxGUI) remux(ctx context.Context)error{src,title,err:=g.selectedTitle();if err!=nil{return err};out:=strings.TrimSpace(g.outputEntry.Text);if out==""{return errors.New("choose an output folder")};g.saveSettings();tools,err:=ensureTools(ctx,false,g.progressCallback());if err!=nil{return err};final,err:=remuxTitle(ctx,src,title,out,g.preserve.Checked,tools,g.progressCallback());if err!=nil{return err};fyne.Do(func(){dialog.ShowInformation("Remux complete","Created:\n"+final,g.window)});return nil}
func (g *linuxGUI) progressCallback()progressFunc{return func(frac float64,status string){fyne.Do(func(){if frac<0{frac=0};if frac>1{frac=1};g.progress.SetValue(frac);if status!=""{g.status.SetText(status)}})}}
func (g *linuxGUI) setProgress(frac float64,status string){g.progress.SetValue(frac);g.status.SetText(status)}
func (g *linuxGUI) invalidateTitles(){g.mu.Lock();g.titles=nil;g.titlesSource="";g.mu.Unlock();g.titleSelect.ClearSelected();g.titleSelect.Options=nil;g.titleSelect.Refresh()}
func (g *linuxGUI) showError(err error){if err!=nil{dialog.ShowError(err,g.window)}}
func (g *linuxGUI) showAbout(){dialog.ShowInformation("About MattMux",fmt.Sprintf("MattMux %s\n\nLinux desktop + CLI DVD-to-MKV remuxer.\n\nMattMux first uses compatible system ffmpeg/ffprobe tools. If FFmpeg does not expose the dvdvideo demuxer, a pinned SHA-256-verified fallback is prepared in your user cache. MediaInfo is optional.\n\nMattMux does not bypass DVD copy protection such as CSS.",appVersion),g.window)}
func firstLine(s string)string{if i:=strings.IndexByte(s,'\n');i>=0{return s[:i]};return s}
func defaultLinuxOutputDir()string{home,err:=os.UserHomeDir();if err!=nil{return "."};videos:=filepath.Join(home,"Videos");if st,err:=os.Stat(videos);err==nil&&st.IsDir(){return videos};return home}
func linuxSettingsPath()string{root,err:=os.UserConfigDir();if err!=nil||root==""{home,_:=os.UserHomeDir();root=filepath.Join(home,".config")};dir:=filepath.Join(root,"mattmux");_ = os.MkdirAll(dir,0700);return filepath.Join(dir,"settings.json")}
func loadLinuxSettings()linuxSettings{s:=linuxSettings{PreserveChapters:true};if b,err:=os.ReadFile(linuxSettingsPath());err==nil{_ = json.Unmarshal(b,&s)};return s}
func (g *linuxGUI) saveSettings(){if g.outputEntry==nil||g.preserve==nil{return};s:=linuxSettings{OutputDir:strings.TrimSpace(g.outputEntry.Text),PreserveChapters:g.preserve.Checked};b,err:=json.MarshalIndent(s,"","  ");if err!=nil{return};path:=linuxSettingsPath();tmp:=path+".tmp";if os.WriteFile(tmp,b,0600)==nil{_ = os.Rename(tmp,path)}}
