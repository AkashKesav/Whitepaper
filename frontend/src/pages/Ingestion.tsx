import React, { useState, useCallback } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { cn } from '@/lib/utils';
import {
    Upload, FileText, Database, Activity, Check, X,
    Clock, ArrowUpRight, RefreshCw, Pause, Play, Trash2,
    File, FileJson, FileCode, AlertCircle
} from 'lucide-react';

interface IngestionJob {
    id: string;
    filename: string;
    status: 'pending' | 'processing' | 'completed' | 'failed';
    progress: number;
    entityCount: number;
    timestamp: Date;
    error?: string;
}

const MOCK_JOBS_EXAMPLE: IngestionJob[] = [
    { id: '1', filename: 'example_report.pdf', status: 'completed', progress: 100, entityCount: 42, timestamp: new Date() },
];

export const Ingestion: React.FC = () => {
    const [isDragging, setIsDragging] = useState(false);
    const [jobs, setJobs] = useState<IngestionJob[]>([]); // Start empty
    const [pipelineActive, setPipelineActive] = useState(true);

    const handleDragOver = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setIsDragging(true);
    }, []);

    const handleDragLeave = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setIsDragging(false);
    }, []);

    const handleDrop = useCallback(async (e: React.DragEvent) => {
        e.preventDefault();
        setIsDragging(false);

        const files = Array.from(e.dataTransfer.files);

        for (const file of files) {
            const jobId = Date.now().toString() + Math.random().toString().slice(2, 5);

            // 1. Add pending job
            const newJob: IngestionJob = {
                id: jobId,
                filename: file.name,
                status: 'processing',
                progress: 10,
                entityCount: 0,
                timestamp: new Date(),
            };
            setJobs(prev => [newJob, ...prev]);

            try {
                // 2. Upload to Backend
                // Simulate progress
                const progressInterval = setInterval(() => {
                    setJobs(prev => prev.map(j =>
                        j.id === jobId && j.progress < 90
                            ? { ...j, progress: j.progress + 10 }
                            : j
                    ));
                }, 500);

                const result = await api.uploadDocument(file);

                clearInterval(progressInterval);

                // 3. Update success
                setJobs(prev => prev.map(j =>
                    j.id === jobId
                        ? {
                            ...j,
                            status: 'completed',
                            progress: 100,
                            entityCount: result.stats?.chunks || 0, // Vector tree chunks count
                        }
                        : j
                ));
            } catch (err: any) {
                // 4. Update failure
                console.error("Ingestion failed:", err);
                setJobs(prev => prev.map(j =>
                    j.id === jobId
                        ? {
                            ...j,
                            status: 'failed',
                            progress: 0,
                            error: err.message || "Upload failed"
                        }
                        : j
                ));
            }
        }
    }, []);

    const getStatusIcon = (status: IngestionJob['status']) => {
        switch (status) {
            case 'completed': return <Check className="w-4 h-4 text-green-400" />;
            case 'processing': return <RefreshCw className="w-4 h-4 text-blue-400 animate-spin" />;
            case 'pending': return <Clock className="w-4 h-4 text-yellow-400" />;
            case 'failed': return <AlertCircle className="w-4 h-4 text-red-400" />;
        }
    };

    const getFileIcon = (filename: string) => {
        if (filename.endsWith('.json') || filename.endsWith('.jsonl')) {
            return <FileJson className="w-5 h-5 text-yellow-400" />;
        }
        if (filename.endsWith('.csv')) {
            return <FileText className="w-5 h-5 text-green-400" />;
        }
        return <File className="w-5 h-5 text-white/40" />;
    };

    const stats = {
        total: jobs.length,
        completed: jobs.filter(j => j.status === 'completed').length,
        processing: jobs.filter(j => j.status === 'processing').length,
        totalEntities: jobs.reduce((sum, j) => sum + j.entityCount, 0),
    };

    return (
        <div className="min-h-screen bg-[#1C1C1E] p-8">
            <div className="max-w-5xl mx-auto">
                {/* Header */}
                <div className="flex items-center justify-between mb-8">
                    <div>
                        <h1 className="text-2xl font-semibold text-white">Data Ingestion</h1>
                        <p className="text-sm text-white/50 mt-1">Upload and manage memory data sources</p>
                    </div>
                    <button
                        onClick={() => setPipelineActive(!pipelineActive)}
                        className={cn(
                            "flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-medium transition-colors",
                            pipelineActive
                                ? "bg-green-500/20 text-green-400 hover:bg-green-500/30"
                                : "bg-white/10 text-white/60 hover:bg-white/15"
                        )}
                    >
                        {pipelineActive ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                        Pipeline {pipelineActive ? 'Active' : 'Paused'}
                    </button>
                </div>

                {/* Stats */}
                <div className="grid grid-cols-4 gap-4 mb-8">
                    {[
                        { label: 'Total Jobs', value: stats.total, color: 'text-white' },
                        { label: 'Completed', value: stats.completed, color: 'text-green-400' },
                        { label: 'Processing', value: stats.processing, color: 'text-blue-400' },
                        { label: 'Entities Indexed', value: stats.totalEntities, color: 'text-purple-400' },
                    ].map((stat) => (
                        <div key={stat.label} className="p-4 bg-white/5 rounded-2xl">
                            <p className="text-xs text-white/50 uppercase tracking-wide">{stat.label}</p>
                            <p className={cn("text-2xl font-semibold mt-1", stat.color)}>{stat.value}</p>
                        </div>
                    ))}
                </div>

                {/* Upload Zone */}
                <div
                    onDragOver={handleDragOver}
                    onDragLeave={handleDragLeave}
                    onDrop={handleDrop}
                    className={cn(
                        "border-2 border-dashed rounded-2xl p-12 text-center transition-all mb-8",
                        isDragging
                            ? "border-blue-500 bg-blue-500/10"
                            : "border-white/20 hover:border-white/40"
                    )}
                >
                    <Upload className={cn(
                        "w-12 h-12 mx-auto mb-4",
                        isDragging ? "text-blue-400" : "text-white/30"
                    )} />
                    <p className="text-white/80 mb-2">
                        {isDragging ? 'Drop files here' : 'Drag and drop files to ingest'}
                    </p>
                    <p className="text-sm text-white/40 mb-4">
                        Supports JSON, JSONL, and CSV formats
                    </p>
                    <button className="px-6 py-2.5 bg-blue-500 hover:bg-blue-600 rounded-xl text-sm text-white font-medium transition-colors">
                        Browse Files
                    </button>
                </div>

                {/* Jobs List */}
                <div>
                    <h2 className="text-lg font-medium text-white mb-4">Recent Ingestions</h2>
                    <div className="space-y-2">
                        {jobs.map((job) => (
                            <div
                                key={job.id}
                                className="flex items-center gap-4 p-4 bg-white/5 rounded-xl hover:bg-white/10 transition-colors"
                            >
                                {getFileIcon(job.filename)}

                                <div className="flex-1 min-w-0">
                                    <div className="flex items-center gap-2">
                                        <span className="text-sm font-medium text-white truncate">
                                            {job.filename}
                                        </span>
                                        {getStatusIcon(job.status)}
                                    </div>
                                    {job.status === 'processing' && (
                                        <div className="mt-2 h-1.5 bg-white/10 rounded-full overflow-hidden">
                                            <div
                                                className="h-full bg-blue-500 rounded-full transition-all"
                                                style={{ width: `${job.progress}%` }}
                                            />
                                        </div>
                                    )}
                                    {job.error && (
                                        <p className="text-xs text-red-400 mt-1">{job.error}</p>
                                    )}
                                </div>

                                <div className="text-right">
                                    {job.entityCount > 0 && (
                                        <p className="text-sm text-white/60">
                                            {job.entityCount} entities
                                        </p>
                                    )}
                                    <p className="text-xs text-white/40 mt-0.5">
                                        {job.timestamp.toLocaleTimeString()}
                                    </p>
                                </div>

                                <button className="p-2 hover:bg-white/10 rounded-lg transition-colors">
                                    <Trash2 className="w-4 h-4 text-white/40 hover:text-red-400" />
                                </button>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
};

export default Ingestion;
